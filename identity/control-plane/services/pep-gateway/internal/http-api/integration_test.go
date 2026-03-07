package httpapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/testutil"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/pep-gateway/internal/storage"
)

type failingInvocationStore struct{}

func (f failingInvocationStore) LastInvocationHash(ctx context.Context, tenant uuid.UUID) (string, error) {
	return "", nil
}

func (f failingInvocationStore) InsertInvocationReceiptIdempotent(ctx context.Context, tenant uuid.UUID, requestID string, decisionID *uuid.UUID, toolName string, method string, path string, outcome string, statusCode *int, latencyMs int, body json.RawMessage, traceID string, spanID string, since time.Time, chainScope string) (receipts.IdempotencyOutcome, error) {
	return receipts.IdempotencyConflict, receipts.ErrReceiptSigningUnavailable
}

func TestInvocationReceiptTraceIDStored(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}
	tenantID := createTenantPEP(t, db.Pool, "pep-trace-tenant")
	store := dbstore.New(db)

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	tid, _ := trace.TraceIDFromHex(traceID)
	sid, _ := trace.SpanIDFromHex(spanID)

	tracer := sdktrace.NewTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				return protocol.DecisionResponse{
					Decision:   "allow",
					DecisionID: uuid.NewString(),
					Reason:     "ok",
					RequestID:  req.Trace.RequestID,
					TraceID:    req.Trace.TraceID,
					SpanID:     req.Trace.SpanID,
				}
			},
		}},
	}

	upstreamURL, _ := url.Parse("http://upstream.invalid")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}

	handler := handleToolProxy(tracer, logger, store, pdp, proxy, "enforce")

	reqID := "req-pep-trace-001"
	req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
	parentCtx := trace.ContextWithSpanContext(req.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	}))
	req = req.WithContext(parentCtx)
	req.Header.Set("x-umbra-tenant-id", tenantID.String())
	req.Header.Set("x-umbra-request-id", reqID)
	req.Header.Set("x-umbra-actor-id", "test-user")
	req.Header.Set("x-umbra-tool-name", "test-tool")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var storedTraceID, storedSpanID string
	if err := db.Pool.QueryRow(ctx, `
    SELECT trace_id, span_id
    FROM receipts_invocation
    WHERE request_id=$1
    ORDER BY ts DESC
    LIMIT 1`, reqID).Scan(&storedTraceID, &storedSpanID); err != nil {
		t.Fatalf("trace lookup failed: %v", err)
	}
	if storedTraceID != traceID {
		t.Fatalf("expected trace_id %s, got %s", traceID, storedTraceID)
	}
	if storedSpanID == "" {
		t.Fatalf("expected span_id to be set")
	}
}

func TestReceiptChainTraceIDConsistency(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}

	tenantID := createTenantPEP(t, db.Pool, "pep-chain-tenant")

	pdpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/decision" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req protocol.DecisionRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Trace == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decisionID := uuid.New()
		type decisionBody struct {
			Actor      protocol.Actor `json:"actor"`
			Tool       protocol.Tool  `json:"tool"`
			Decision   string         `json:"decision"`
			PolicyHash string         `json:"policy_hash"`
			RequestID  string         `json:"request_id,omitempty"`
			TraceID    string         `json:"trace_id,omitempty"`
			SpanID     string         `json:"span_id,omitempty"`
		}
		bodyBytes, err := receipts.CanonicalJSON(decisionBody{
			Actor:      req.Actor,
			Tool:       req.Tool,
			Decision:   "allow",
			PolicyHash: "policy-hash",
			RequestID:  req.Trace.RequestID,
			TraceID:    req.Trace.TraceID,
			SpanID:     req.Trace.SpanID,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		hash := receipts.HashBytes(bodyBytes)
		_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, request_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			tenantID, decisionID, req.Trace.RequestID, "policy-hash", "allow", bodyBytes, bodyBytes, nil, hash, req.Trace.TraceID, req.Trace.SpanID,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := protocol.DecisionResponse{
			Decision:      "allow",
			DecisionID:    decisionID.String(),
			PolicyVersion: 1,
			PolicyHash:    "policy-hash",
			RequestID:     req.Trace.RequestID,
			TraceID:       req.Trace.TraceID,
			SpanID:        req.Trace.SpanID,
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer pdpSrv.Close()

	tracer := sdktrace.NewTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	pdp := &PDPClient{
		BaseURL: pdpSrv.URL,
		Client:  &http.Client{Timeout: 3 * time.Second},
	}

	upstreamURL, _ := url.Parse("http://upstream.invalid")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}
	store := dbstore.New(db)

	handler := handleToolProxy(tracer, logger, store, pdp, proxy, "enforce")

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	tid, _ := trace.TraceIDFromHex(traceID)
	sid, _ := trace.SpanIDFromHex(spanID)
	reqID := "req-chain-001"

	req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
	parentCtx := trace.ContextWithSpanContext(req.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	}))
	req = req.WithContext(parentCtx)
	req.Header.Set("x-umbra-tenant-id", tenantID.String())
	req.Header.Set("x-umbra-request-id", reqID)
	req.Header.Set("x-umbra-actor-id", "test-user")
	req.Header.Set("x-umbra-tool-name", "test-tool")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var decisionTrace, invTrace string
	if err := db.Pool.QueryRow(ctx, `
    SELECT trace_id FROM receipts_decision WHERE request_id=$1 ORDER BY ts DESC LIMIT 1`, reqID).Scan(&decisionTrace); err != nil {
		t.Fatalf("decision trace lookup failed: %v", err)
	}
	if err := db.Pool.QueryRow(ctx, `
    SELECT trace_id FROM receipts_invocation WHERE request_id=$1 ORDER BY ts DESC LIMIT 1`, reqID).Scan(&invTrace); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("missing invocation receipt")
		}
		t.Fatalf("invocation trace lookup failed: %v", err)
	}
	if decisionTrace != traceID {
		t.Fatalf("expected decision trace_id %s, got %s", traceID, decisionTrace)
	}
	if invTrace != traceID {
		t.Fatalf("expected invocation trace_id %s, got %s", traceID, invTrace)
	}
}

func TestInvocationReceiptSigningEnabled(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}
	tenantID := createTenantPEP(t, db.Pool, "pep-sign-tenant")

	privateKeyPEM, publicKeyPEM := mustGenerateECDSAPEPPTestKey(t)
	signer, err := receipts.NewECDSAP256SignerFromPEM(privateKeyPEM, "key://pep-test")
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	store := dbstore.NewWithSigner(db, signer)

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	tid, _ := trace.TraceIDFromHex(traceID)
	sid, _ := trace.SpanIDFromHex(spanID)

	tracer := sdktrace.NewTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				return protocol.DecisionResponse{
					Decision:   "allow",
					DecisionID: uuid.NewString(),
					Reason:     "ok",
					RequestID:  req.Trace.RequestID,
					TraceID:    req.Trace.TraceID,
					SpanID:     req.Trace.SpanID,
				}
			},
		}},
	}

	upstreamURL, _ := url.Parse("http://upstream.invalid")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}
	handler := handleToolProxy(tracer, logger, store, pdp, proxy, "enforce")

	reqID := "req-pep-sign-1"
	req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
	parentCtx := trace.ContextWithSpanContext(req.Context(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	}))
	req = req.WithContext(parentCtx)
	req.Header.Set("x-umbra-tenant-id", tenantID.String())
	req.Header.Set("x-umbra-request-id", reqID)
	req.Header.Set("x-umbra-actor-id", "test-user")
	req.Header.Set("x-umbra-tool-name", "test-tool")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var hash, alg, kid, signature string
	if err := db.Pool.QueryRow(ctx, `
    SELECT hash, signature_alg, signature_kid, signature
    FROM receipts_invocation
    WHERE request_id=$1
    ORDER BY ts DESC
    LIMIT 1`, reqID).Scan(&hash, &alg, &kid, &signature); err != nil {
		t.Fatalf("signed receipt lookup failed: %v", err)
	}
	if alg != receipts.SignatureAlgECDSAP256SHA256 {
		t.Fatalf("unexpected signature alg: %s", alg)
	}
	if kid != "key://pep-test" {
		t.Fatalf("unexpected signature kid: %s", kid)
	}
	if signature == "" {
		t.Fatal("expected signature")
	}

	publicKey, err := receipts.ParseECDSAPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	if err := receipts.VerifyECDSAP256SignatureHashHex(publicKey, hash, signature); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

func TestPEPRegisterFailsWhenSigningRequiredAndDisabled(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("UMBRA_RECEIPT_SIGNING_ENABLED", "false")
	t.Setenv("UMBRA_RECEIPT_SIGNING_REQUIRED", "true")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	mux := http.NewServeMux()
	err := registerV0(mux, logger)
	if !errors.Is(err, receipts.ErrReceiptSigningUnavailable) {
		t.Fatalf("expected signing unavailable error, got %v", err)
	}
}

func TestHandleToolProxyReturns503OnRequiredSigningFailure(t *testing.T) {
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	store := failingInvocationStore{}
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				return protocol.DecisionResponse{
					Decision:   "allow",
					DecisionID: uuid.NewString(),
					Reason:     "ok",
					RequestID:  req.Trace.RequestID,
					TraceID:    req.Trace.TraceID,
					SpanID:     req.Trace.SpanID,
				}
			},
		}},
	}
	upstreamURL, _ := url.Parse("http://upstream.invalid")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}
	handler := handleToolProxy(tracer, logger, store, pdp, proxy, "enforce")

	req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	req.Header.Set("x-umbra-request-id", "req-pep-signing-fail")
	req.Header.Set("x-umbra-tool-name", "test-tool")
	req.Header.Set("x-umbra-actor-id", "test-user")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d %s", rec.Code, rec.Body.String())
	}

	var errResp protocol.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("parse error response failed: %v", err)
	}
	if errResp.Error.Code != protocol.ErrorCodeReceiptSigningUnavailable {
		t.Fatalf("expected %s, got %s", protocol.ErrorCodeReceiptSigningUnavailable, errResp.Error.Code)
	}
}

func applySchema(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	return testutil.ApplySchemaForTests(t, pool)
}

func createTenantPEP(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO tenants(name) VALUES($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("insert tenant failed: %v", err)
	}
	return id
}

func mustGenerateECDSAPEPPTestKey(t *testing.T) ([]byte, []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privateDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return privatePEM, publicPEM
}
