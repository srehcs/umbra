package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

type captureStore struct {
	last receiptCapture
}

type receiptCapture struct {
	requestID   string
	decisionID  *uuid.UUID
	outcome     string
	enforcement string
	pdpStatus   string
	body        invocationReceiptBody
}

func (s *captureStore) LastInvocationHash(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}

func (s *captureStore) InsertInvocationReceipt(_ context.Context, _ uuid.UUID, decisionID *uuid.UUID, requestID string, _ string, _ string, _ string, outcome string, _ *int, _ int, body json.RawMessage, _ string, _ string, _ string, _ string) error {
	var parsed invocationReceiptBody
	_ = json.Unmarshal(body, &parsed)
	s.last = receiptCapture{
		requestID:   requestID,
		decisionID:  decisionID,
		outcome:     outcome,
		enforcement: parsed.Enforcement,
		pdpStatus:   parsed.PDPStatus,
		body:        parsed,
	}
	return nil
}

func newTestHandler(t *testing.T, pepMode string, pdpURL string, upstreamURL string, store invocationStore) *mcpHandler {
	t.Helper()
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return &mcpHandler{
		tracer:        tracer,
		logger:        logger,
		store:         store,
		pdp:           &PDPClient{BaseURL: pdpURL, Client: &http.Client{Timeout: 300 * time.Millisecond}},
		toolClient:    &http.Client{Timeout: 300 * time.Millisecond},
		pepMode:       pepMode,
		upstreamURL:   upstreamURL,
		defaultTenant: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		actorID:       "user-1",
		actorRoles:    []string{"developer"},
		serverName:    "mcp.test",
	}
}

func TestMCPAllowForwardedReceipt(t *testing.T) {
	var pdpRequestID string

	pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.DecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("pdp request decode failed: %v", err)
		}
		pdpRequestID = req.Trace.RequestID
		resp := protocol.DecisionResponse{
			Decision:   "allow",
			DecisionID: uuid.NewString(),
			Reason:     "ok",
			RequestID:  req.Trace.RequestID,
			TraceID:    req.Trace.TraceID,
			SpanID:     req.Trace.SpanID,
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer pdpServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	}))
	defer upstreamServer.Close()

	store := &captureStore{}
	h := newTestHandler(t, "enforce", pdpServer.URL, upstreamServer.URL, store)

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"demo.tool","arguments":{"a":1,"b":2}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	h.handleMCP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}
	if pdpRequestID == "" {
		t.Fatal("expected request_id sent to pdp")
	}
	if store.last.requestID == "" {
		t.Fatal("expected request_id in receipt")
	}
	if store.last.requestID != pdpRequestID {
		t.Fatalf("request_id mismatch: %s vs %s", store.last.requestID, pdpRequestID)
	}
	if store.last.enforcement != "forwarded" {
		t.Fatalf("expected enforcement forwarded, got %s", store.last.enforcement)
	}
	if store.last.body.PDPLatencyMs < 0 {
		t.Fatalf("expected pdp latency set")
	}
}

func TestMCPDenyEnforceBlocked(t *testing.T) {
	var upstreamCalls int32

	pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := protocol.DecisionResponse{
			Decision:   "deny",
			DecisionID: uuid.NewString(),
			Reason:     "nope",
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer pdpServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamCalls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamServer.Close()

	store := &captureStore{}
	h := newTestHandler(t, "enforce", pdpServer.URL, upstreamServer.URL, store)

	reqBody := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"demo.tool"}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	h.handleMCP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 403, got %d: %s", res.StatusCode, string(body))
	}
	if atomic.LoadInt32(&upstreamCalls) != 0 {
		t.Fatalf("expected no upstream calls")
	}
	if store.last.outcome != "denied" {
		t.Fatalf("expected denied outcome, got %s", store.last.outcome)
	}
	if store.last.enforcement != "blocked" {
		t.Fatalf("expected blocked enforcement, got %s", store.last.enforcement)
	}
}

func TestMCPPDPUnavailableByMode(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`))
	}))
	defer upstreamServer.Close()

	tests := []struct {
		name          string
		pepMode       string
		expectCode    int
		expectForward bool
	}{
		{name: "enforce", pepMode: "enforce", expectCode: http.StatusServiceUnavailable, expectForward: false},
		{name: "observe", pepMode: "observe", expectCode: http.StatusOK, expectForward: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &captureStore{}
			h := newTestHandler(t, tt.pepMode, "http://127.0.0.1:1", upstreamServer.URL, store)

			reqBody := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"demo.tool"}}`)
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
			rec := httptest.NewRecorder()

			h.handleMCP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.expectCode {
				body, _ := io.ReadAll(res.Body)
				t.Fatalf("expected %d, got %d: %s", tt.expectCode, res.StatusCode, string(body))
			}
			if store.last.pdpStatus != "unavailable" {
				t.Fatalf("expected pdp_status unavailable, got %s", store.last.pdpStatus)
			}
			if tt.expectForward && store.last.enforcement != "forwarded" {
				t.Fatalf("expected forwarded enforcement")
			}
			if !tt.expectForward && store.last.enforcement != "blocked" {
				t.Fatalf("expected blocked enforcement")
			}
		})
	}
}
