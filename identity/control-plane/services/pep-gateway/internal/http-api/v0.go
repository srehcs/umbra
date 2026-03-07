package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/pep-gateway/internal/storage"
)

type PDPClient struct {
	BaseURL string
	Client  *http.Client
}

const maxPDPResponseBytes = 1 << 20

type invocationStore interface {
	LastInvocationHash(ctx context.Context, tenant uuid.UUID) (string, error)
	InsertInvocationReceiptIdempotent(ctx context.Context, tenant uuid.UUID, requestID string, decisionID *uuid.UUID, toolName string, method string, path string, outcome string, statusCode *int, latencyMs int, body json.RawMessage, traceID string, spanID string, since time.Time, chainScope string) (receipts.IdempotencyOutcome, error)
}

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return e.Body }

func (c *PDPClient) Decide(ctx context.Context, payload protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
	var out protocol.DecisionResponse

	b, err := json.Marshal(payload)
	if err != nil {
		return out, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/decision", bytes.NewReader(b))
	if err != nil {
		return out, 0, err
	}
	req.Header.Set("content-type", "application/json")

	res, err := c.Client.Do(req)
	if err != nil {
		return out, 0, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, maxPDPResponseBytes))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return out, res.StatusCode, &httpError{Status: res.StatusCode, Body: string(body)}
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, res.StatusCode, err
	}
	return out, res.StatusCode, nil
}

type demoInvokeRequest struct {
	Tool   string `json:"tool"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Actor  struct {
		ID    string   `json:"id"`
		Roles []string `json:"roles"`
	} `json:"actor"`
}

type invocationReceiptBody struct {
	Tool          string            `json:"tool"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	PolicyHash    string            `json:"policy_hash,omitempty"`
	PolicyVersion int               `json:"policy_version,omitempty"`
	Decision      string            `json:"decision.result,omitempty"`
	Outcome       string            `json:"outcome"`
	StatusCode    *int              `json:"status_code,omitempty"`
	LatencyMs     int               `json:"latency_ms"`
	Meta          map[string]string `json:"meta,omitempty"`
	PDPStatus     string            `json:"pdp.status,omitempty"`
	PDPErrorCode  string            `json:"pdp.error_code,omitempty"`
	StartedAt     string            `json:"started_at"` // RFC3339
	RequestID     string            `json:"request_id,omitempty"`
	PEPMode       string            `json:"pep.mode,omitempty"`
	Enforcement   string            `json:"enforcement.outcome,omitempty"`
}

func writeErrorResponse(w http.ResponseWriter, status int, code, message, requestID, decisionID, traceID string) {
	protocol.WriteErrorResponse(w, status, code, message, requestID, decisionID, traceID, nil)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeErrorResponse(w, http.StatusMethodNotAllowed, protocol.ErrorCodeMethodNotAllowed, "method not allowed", "", "", "")
}

func writeInvalidJSON(w http.ResponseWriter) {
	writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidJSON, "invalid json", "", "", "")
}

func writeInvalidTenant(w http.ResponseWriter) {
	writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidTenant, "missing/invalid x-umbra-tenant-id", "", "", "")
}

func registerV0(mux *http.ServeMux, logger *slog.Logger) error {
	tracer := otel.Tracer("umbra.pep")

	pepMode := getenv("PEP_MODE", "observe")
	if pepMode != "enforce" && pepMode != "observe" {
		logger.Warn("invalid PEP_MODE, defaulting to observe", "value", pepMode)
		pepMode = "observe"
	}

	pdp := &PDPClient{
		BaseURL: getenv("PDP_URL", "http://pdp:8081"),
		Client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}

	db, err := stor.Connect(context.Background(), getenv("DATABASE_URL", "postgres://umbra:umbra@postgres:5432/umbra?sslmode=disable"))
	if err != nil {
		logger.Error("db connect failed", "err", err)
	}
	var store *dbstore.Store
	if db != nil {
		signer, signerPolicy, signErr := receipts.NewSignerFromEnvWithPolicy()
		if signErr != nil {
			if signerPolicy.Required || receipts.IsReceiptSigningUnavailable(signErr) {
				return signErr
			}
			logger.Error("receipt signer init failed; continuing without signing", "err", signErr)
			store = dbstore.New(db)
		} else if signer != nil {
			logger.Info("receipt signing enabled for pep-gateway")
			store = dbstore.NewWithSignerPolicy(db, signer, signerPolicy.Required)
		} else {
			store = dbstore.New(db)
		}
	}

	upstreamURL, err := url.Parse(getenv("UPSTREAM_URL", "http://upstream-sample:8090"))
	if err != nil {
		logger.Error("invalid upstream url", "err", err)
		upstreamURL = &url.URL{Scheme: "http", Host: "upstream-sample:8090"}
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// V0 convenience endpoint: POST /demo (JSON) to exercise the enforcement flow.
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}

		var in demoInvokeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
			writeInvalidJSON(w)
			return
		}

		// Translate to tool proxy call.
		r2 := r.Clone(r.Context())
		r2.Method = strings.ToUpper(strings.TrimSpace(in.Method))
		if r2.Method == "" {
			r2.Method = http.MethodGet
		}
		r2.URL.Path = "/tool" + in.Path

		// Attach actor hints.
		r2.Header.Set("x-umbra-tool-name", in.Tool)
		r2.Header.Set("x-umbra-actor-id", in.Actor.ID)
		r2.Header.Set("x-umbra-actor-roles", strings.Join(in.Actor.Roles, ","))

		handleToolProxy(tracer, logger, store, pdp, proxy, pepMode).ServeHTTP(w, r2)
	})

	// Primary V0 enforcement surface: /tool/* forwarded to an upstream, with delegated PDP decision.
	mux.Handle("/tool/", handleToolProxy(tracer, logger, store, pdp, proxy, pepMode))
	return nil
}

// handleToolProxy enforces a PDP decision before proxying to upstream.
func handleToolProxy(tracer trace.Tracer, logger *slog.Logger, store invocationStore, pdp *PDPClient, proxy *httputil.ReverseProxy, pepMode string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("x-umbra-tenant-id")
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil || tenantID == uuid.Nil {
			writeInvalidTenant(w)
			return
		}

		reqID := r.Header.Get("x-umbra-request-id")
		if reqID == "" {
			reqID = uuid.NewString()
			r.Header.Set("x-umbra-request-id", reqID)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()

		ctx, span := tracer.Start(ctx, "pep.enforce")
		defer span.End()

		toolName := strings.TrimSpace(r.Header.Get("x-umbra-tool-name"))
		if toolName == "" {
			toolName = "demo.http"
		}

		actorID := strings.TrimSpace(r.Header.Get("x-umbra-actor-id"))
		actorSource := "dev"
		if actorID == "" {
			actorID = "user-1"
		} else {
			actorSource = "header"
		}
		roles := parseCSV(r.Header.Get("x-umbra-actor-roles"))
		if len(roles) == 0 {
			roles = []string{"developer"}
		} else {
			actorSource = "header"
		}

		// The upstream path we proxy (strip /tool prefix).
		upath := strings.TrimPrefix(r.URL.Path, "/tool")
		if upath == "" {
			upath = "/"
		}

		sc := trace.SpanContextFromContext(ctx)
		var traceCtx *protocol.TraceContext
		if sc.IsValid() {
			traceCtx = &protocol.TraceContext{
				RequestID: reqID,
				TraceID:   sc.TraceID().String(),
				SpanID:    sc.SpanID().String(),
			}
		} else {
			traceCtx = &protocol.TraceContext{RequestID: reqID}
		}

		reqLogger := logger.With("request_id", reqID)
		if sc.IsValid() {
			reqLogger = reqLogger.With("trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
		}

		payload := protocol.DecisionRequest{
			Tenant: protocol.TenantContext{TenantID: tenantID.String()},
			Actor:  protocol.Actor{Type: "human", ID: actorID, Roles: roles, Source: actorSource},
			Tool: protocol.Tool{
				Name:     toolName,
				Method:   r.Method,
				Endpoint: upath,
			},
			Trace: traceCtx,
		}

		span.SetAttributes(
			attribute.String("umbra.tenant_id", tenantID.String()),
			attribute.String("umbra.actor_id", actorID),
			attribute.String("umbra.tool", toolName),
			attribute.String("http.method", r.Method),
			attribute.String("http.route", upath),
			attribute.String("pep.mode", pepMode),
			attribute.String("umbra.request_id", reqID),
		)

		started := time.Now()
		decision, status, err := pdp.Decide(ctx, payload)

		if err != nil {
			pdpStatus, pdpErrorCode := classifyPDPError(err, status)
			reqLogger.Error("pdp decide failed", "err", err, "status", status)
			if pepMode == "observe" {
				reqLogger.Info("pdp unavailable in observe mode, forwarding", "tenant", tenantID.String())
				r.URL.Path = upath
				r.Host = ""
				proxy.ModifyResponse = func(resp *http.Response) error {
					lat := int(time.Since(started).Milliseconds())
					return writeInvocationReceipt(ctx, reqLogger, store, tenantID, nil, reqID, toolName, r.Method, upath, "", 0, "error", intPtr(resp.StatusCode), lat,
						map[string]string{"stage": "pdp"}, "unavailable", pdpStatus, pdpErrorCode, started, pepMode, "forwarded")
				}
				proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
					if errors.Is(e, receipts.ErrReceiptSigningUnavailable) {
						writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, "", traceCtx.TraceID)
						return
					}
					lat := int(time.Since(started).Milliseconds())
					reqLogger.Error("upstream error in observe mode (pdp unavailable)", "err", e)
					if recErr := writeInvocationReceipt(ctx, reqLogger, store, tenantID, nil, reqID, toolName, r.Method, upath, "", 0, "error", intPtr(http.StatusBadGateway), lat,
						map[string]string{"stage": "pdp"}, "unavailable", pdpStatus, pdpErrorCode, started, pepMode, "forwarded"); errors.Is(recErr, receipts.ErrReceiptSigningUnavailable) {
						writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, "", traceCtx.TraceID)
						return
					}
					writeErrorResponse(rw, http.StatusBadGateway, protocol.ErrorCodeUpstreamError, "upstream error", reqID, "", traceCtx.TraceID)
				}
				proxy.ServeHTTP(w, r)
				return
			}

			lat := int(time.Since(started).Milliseconds())
			if recErr := writeInvocationReceipt(ctx, reqLogger, store, tenantID, nil, reqID, toolName, r.Method, upath, "", 0, "denied", intPtr(http.StatusServiceUnavailable), lat,
				map[string]string{"stage": "pdp"}, "unavailable", pdpStatus, pdpErrorCode, started, pepMode, "blocked"); errors.Is(recErr, receipts.ErrReceiptSigningUnavailable) {
				writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, "", traceCtx.TraceID)
				return
			}
			writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodePolicyUnavailable, "pdp unavailable", reqID, "", traceCtx.TraceID)
			return
		}

		decisionID, _ := uuid.Parse(decision.DecisionID)
		span.SetAttributes(attribute.String("umbra.decision_id", decision.DecisionID))
		reqLogger = reqLogger.With("decision_id", decision.DecisionID)
		reqLogger.Info("pdp decision received", "decision", decision.Decision)
		if strings.ToLower(decision.Decision) != "allow" {
			// Decision is DENY
			if pepMode == "observe" {
				// Observe mode: forward the request but record as denied/forwarded
				reqLogger.Info("deny decision in observe mode, forwarding", "tenant", tenantID.String())
				r.URL.Path = upath
				r.Host = "" // let reverse proxy set Host appropriately
				proxy.ModifyResponse = func(resp *http.Response) error {
					lat := int(time.Since(started).Milliseconds())
					return writeInvocationReceipt(ctx, reqLogger, store, tenantID, &decisionID, reqID, toolName, r.Method, upath, decision.PolicyHash, decision.PolicyVersion, "denied", intPtr(resp.StatusCode), lat,
						map[string]string{"reason": decision.Reason}, decision.Decision, "ok", "", started, pepMode, "forwarded")
				}
				proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
					if errors.Is(e, receipts.ErrReceiptSigningUnavailable) {
						writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, decision.DecisionID, traceCtx.TraceID)
						return
					}
					lat := int(time.Since(started).Milliseconds())
					reqLogger.Error("upstream error in observe mode", "err", e)
					if recErr := writeInvocationReceipt(ctx, reqLogger, store, tenantID, &decisionID, reqID, toolName, r.Method, upath, decision.PolicyHash, decision.PolicyVersion, "denied", intPtr(http.StatusBadGateway), lat,
						map[string]string{"reason": decision.Reason}, decision.Decision, "ok", "", started, pepMode, "forwarded"); errors.Is(recErr, receipts.ErrReceiptSigningUnavailable) {
						writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, decision.DecisionID, traceCtx.TraceID)
						return
					}
					writeErrorResponse(rw, http.StatusBadGateway, protocol.ErrorCodeUpstreamError, "upstream error", reqID, decision.DecisionID, traceCtx.TraceID)
				}
				proxy.ServeHTTP(w, r)
				return
			} else {
				// Enforce mode: block the request
				reqLogger.Info("deny decision in enforce mode, blocking", "tenant", tenantID.String())
				lat := int(time.Since(started).Milliseconds())
				if recErr := writeInvocationReceipt(ctx, reqLogger, store, tenantID, &decisionID, reqID, toolName, r.Method, upath, decision.PolicyHash, decision.PolicyVersion, "denied", intPtr(http.StatusForbidden), lat,
					map[string]string{"reason": decision.Reason}, decision.Decision, "ok", "", started, pepMode, "blocked"); errors.Is(recErr, receipts.ErrReceiptSigningUnavailable) {
					writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, decision.DecisionID, traceCtx.TraceID)
					return
				}

				// Return structured error response
				writeErrorResponse(w, http.StatusForbidden, protocol.ErrorCodePolicyDenied, decision.Reason, reqID, decision.DecisionID, traceCtx.TraceID)
				return
			}
		}

		// Allowed: proxy to upstream
		r.URL.Path = upath
		r.Host = "" // let reverse proxy set Host appropriately
		proxy.ModifyResponse = func(resp *http.Response) error {
			lat := int(time.Since(started).Milliseconds())
			return writeInvocationReceipt(ctx, reqLogger, store, tenantID, &decisionID, reqID, toolName, r.Method, upath, decision.PolicyHash, decision.PolicyVersion, "success", intPtr(resp.StatusCode), lat,
				map[string]string{"upstream": proxyURLHost(proxy)}, decision.Decision, "ok", "", started, pepMode, "forwarded")
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
			if errors.Is(e, receipts.ErrReceiptSigningUnavailable) {
				writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, decision.DecisionID, traceCtx.TraceID)
				return
			}
			lat := int(time.Since(started).Milliseconds())
			reqLogger.Error("upstream error", "err", e)
			if recErr := writeInvocationReceipt(ctx, reqLogger, store, tenantID, &decisionID, reqID, toolName, r.Method, upath, decision.PolicyHash, decision.PolicyVersion, "error", intPtr(http.StatusBadGateway), lat,
				map[string]string{"upstream": proxyURLHost(proxy)}, decision.Decision, "ok", "", started, pepMode, "forwarded"); errors.Is(recErr, receipts.ErrReceiptSigningUnavailable) {
				writeErrorResponse(rw, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", reqID, decision.DecisionID, traceCtx.TraceID)
				return
			}
			writeErrorResponse(rw, http.StatusBadGateway, protocol.ErrorCodeUpstreamError, "upstream error", reqID, decision.DecisionID, traceCtx.TraceID)
		}

		proxy.ServeHTTP(w, r)
	})
}

func writeInvocationReceipt(ctx context.Context, logger *slog.Logger, store invocationStore, tenant uuid.UUID, decisionID *uuid.UUID, requestID string,
	toolName, method, path, policyHash string, policyVersion int, outcome string, statusCode *int, latencyMs int, meta map[string]string, decisionResult, pdpStatus, pdpErrorCode string, started time.Time, pepMode, enforcement string) error {

	if store == nil {
		logger.Warn("invocation receipt skipped (no store)")
		return nil
	}

	rb := invocationReceiptBody{
		Tool:          toolName,
		Method:        method,
		Path:          path,
		PolicyHash:    policyHash,
		PolicyVersion: policyVersion,
		Decision:      decisionResult,
		Outcome:       outcome,
		StatusCode:    statusCode,
		LatencyMs:     latencyMs,
		Meta:          meta,
		PDPStatus:     pdpStatus,
		PDPErrorCode:  pdpErrorCode,
		StartedAt:     started.UTC().Format(time.RFC3339),
		RequestID:     requestID,
		PEPMode:       pepMode,
		Enforcement:   enforcement,
	}

	bodyBytes, err := receipts.CanonicalJSON(rb)
	if err != nil {
		logger.Error("receipt canonical json failed", "err", err)
		return nil
	}

	sc := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if sc.IsValid() {
		traceID, spanID = sc.TraceID().String(), sc.SpanID().String()
	}

	since := receipts.RequestIDDedupeSince(time.Now().UTC(), requestIDDedupeWindow(logger))
	outcomeResult, err := store.InsertInvocationReceiptIdempotent(ctx, tenant, requestID, decisionID, toolName, method, path, outcome, statusCode, latencyMs, bodyBytes, traceID, spanID, since, receiptChainLockScope(logger))
	if err != nil {
		if errors.Is(err, receipts.ErrReceiptSigningUnavailable) {
			return err
		}
		logger.Error("receipt insert failed", "err", err)
		return nil
	}
	switch outcomeResult {
	case receipts.IdempotencyReplayed:
		logger.Info("receipt idempotency replay", "request_id", requestID)
	case receipts.IdempotencyConflict:
		logger.Error("receipt idempotency conflict", "request_id", requestID)
	}
	return nil
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func proxyURLHost(_ *httputil.ReverseProxy) string {
	// Best-effort: keep it simple in V0
	return getenv("UPSTREAM_URL", "http://upstream-sample:8090")
}

func intPtr(v int) *int { return &v }

var dedupeWindowOnce sync.Once
var dedupeWindow time.Duration
var chainScopeOnce sync.Once
var chainScope string

func requestIDDedupeWindow(logger *slog.Logger) time.Duration {
	dedupeWindowOnce.Do(func() {
		window, err := receipts.ResolveRequestIDDedupeWindow(getenv("UMBRA_REQUEST_ID_DEDUPE_WINDOW", ""))
		if err != nil {
			logger.Warn("invalid UMBRA_REQUEST_ID_DEDUPE_WINDOW; using default", "err", err)
		}
		dedupeWindow = window
	})
	if dedupeWindow == 0 {
		return receipts.DefaultRequestIDDedupeWindow
	}
	return dedupeWindow
}

func receiptChainLockScope(logger *slog.Logger) string {
	chainScopeOnce.Do(func() {
		scope, err := receipts.ResolveChainLockScope(getenv("UMBRA_RECEIPT_CHAIN_LOCK_SCOPE", ""))
		if err != nil {
			logger.Warn("invalid UMBRA_RECEIPT_CHAIN_LOCK_SCOPE; using default", "err", err)
		}
		chainScope = scope
	})
	if chainScope == "" {
		return "tenant"
	}
	return chainScope
}

func classifyPDPError(err error, status int) (string, string) {
	if err == nil {
		return "ok", ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout", protocol.ErrorCodePolicyUnavailable
	}
	if status >= 500 || status == 0 {
		return "unavailable", protocol.ErrorCodePolicyUnavailable
	}
	return "error", protocol.ErrorCodePolicyUnavailable
}

func getenv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
