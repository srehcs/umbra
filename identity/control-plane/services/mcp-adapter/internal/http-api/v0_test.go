package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	rawBody     json.RawMessage
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
		rawBody:     body,
	}
	return nil
}

func newTestHandler(t *testing.T, pepMode string, pdpTransport http.RoundTripper, toolTransport http.RoundTripper, store invocationStore) *mcpHandler {
	t.Helper()
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	if pdpTransport == nil {
		pdpTransport = http.DefaultTransport
	}
	if toolTransport == nil {
		toolTransport = http.DefaultTransport
	}
	return &mcpHandler{
		tracer:        tracer,
		logger:        logger,
		store:         store,
		pdp:           &PDPClient{BaseURL: "http://pdp.invalid", Client: &http.Client{Timeout: 300 * time.Millisecond, Transport: pdpTransport}},
		toolClient:    &http.Client{Timeout: 300 * time.Millisecond, Transport: toolTransport},
		pepMode:       pepMode,
		upstreamURL:   "http://upstream.invalid",
		defaultTenant: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		defaultActor: actorIdentity{
			ID:     "user-1",
			Type:   "agent",
			Roles:  []string{"developer"},
			Source: "dev",
		},
		serverName: "mcp.test",
	}
}

func TestMCPAllowForwardedReceipt(t *testing.T) {
	var pdpRequestID string

	store := &captureStore{}
	h := newTestHandler(t, "enforce",
		pdpRoundTripper{onRequest: func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
			pdpRequestID = req.Trace.RequestID
			return protocol.DecisionResponse{
				Decision:   "allow",
				DecisionID: uuid.NewString(),
				Reason:     "ok",
				RequestID:  req.Trace.RequestID,
				TraceID:    req.Trace.TraceID,
				SpanID:     req.Trace.SpanID,
			}, http.StatusOK, nil
		}},
		staticRoundTripper{status: http.StatusOK, body: `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`},
		store,
	)

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
	if store.last.body.ActorID == "" || store.last.body.ActorType == "" {
		t.Fatalf("expected actor identity in receipt")
	}
	if store.last.body.ActorRoles == nil {
		t.Fatalf("expected actor roles in receipt")
	}
}

func TestMCPDenyEnforceBlocked(t *testing.T) {
	var upstreamCalls int32

	store := &captureStore{}
	h := newTestHandler(t, "enforce",
		pdpRoundTripper{onRequest: func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
			return protocol.DecisionResponse{
				Decision:   "deny",
				DecisionID: uuid.NewString(),
				Reason:     "nope",
			}, http.StatusOK, nil
		}},
		roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
			atomic.AddInt32(&upstreamCalls, 1)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
		store,
	)

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
			h := newTestHandler(t, tt.pepMode,
				errorRoundTripper{err: context.DeadlineExceeded},
				staticRoundTripper{status: http.StatusOK, body: `{"jsonrpc":"2.0","id":3,"result":{"ok":true}}`},
				store,
			)

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

func TestMCPActorIdentityFromParams(t *testing.T) {
	store := &captureStore{}
	h := newTestHandler(t, "enforce",
		pdpRoundTripper{onRequest: func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
			return protocol.DecisionResponse{
				Decision:   "allow",
				DecisionID: uuid.NewString(),
				Reason:     "ok",
			}, http.StatusOK, nil
		}},
		staticRoundTripper{status: http.StatusOK, body: `{"jsonrpc":"2.0","id":4,"result":{"ok":true}}`},
		store,
	)

	reqBody := []byte(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"demo.tool","actor":{"id":"human-1","type":"human","roles":["admin"],"source":"client"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	h.handleMCP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 200, got %d: %s", res.StatusCode, string(body))
	}
	if store.last.body.ActorID != "human-1" {
		t.Fatalf("expected actor id human-1, got %s", store.last.body.ActorID)
	}
	if store.last.body.ActorType != "human" {
		t.Fatalf("expected actor type human, got %s", store.last.body.ActorType)
	}
	if store.last.body.ActorSource != "client" {
		t.Fatalf("expected actor source client, got %s", store.last.body.ActorSource)
	}
	if len(store.last.body.ActorRoles) != 1 || store.last.body.ActorRoles[0] != "admin" {
		t.Fatalf("expected actor roles admin, got %v", store.last.body.ActorRoles)
	}
}

func TestMCPArgsRedactionInReceipt(t *testing.T) {
	store := &captureStore{}
	h := newTestHandler(t, "enforce",
		pdpRoundTripper{onRequest: func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
			return protocol.DecisionResponse{
				Decision:   "allow",
				DecisionID: uuid.NewString(),
				Reason:     "ok",
			}, http.StatusOK, nil
		}},
		staticRoundTripper{status: http.StatusOK, body: `{"jsonrpc":"2.0","id":5,"result":{"ok":true}}`},
		store,
	)

	reqBody := []byte(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"demo.tool","arguments":{"password":"secret","nested":{"token":"abc"}}}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	h.handleMCP(rec, req)

	if bytes.Contains(store.last.rawBody, []byte("password")) || bytes.Contains(store.last.rawBody, []byte("arguments")) {
		t.Fatalf("expected args redacted from receipt body")
	}
}

func TestMCPContextTooLarge(t *testing.T) {
	store := &captureStore{}
	h := newTestHandler(t, "enforce",
		pdpRoundTripper{onRequest: func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
			return protocol.DecisionResponse{
				Decision:   "allow",
				DecisionID: uuid.NewString(),
				Reason:     "ok",
			}, http.StatusOK, nil
		}},
		staticRoundTripper{status: http.StatusOK, body: `{"jsonrpc":"2.0","id":6,"result":{"ok":true}}`},
		store,
	)

	oversized := strings.Repeat("x", maxServerLen+1)
	reqBody := []byte(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"demo.tool","server":"` + oversized + `"}}`)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	h.handleMCP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 400, got %d: %s", res.StatusCode, string(body))
	}
}

type pdpRoundTripper struct {
	onRequest func(req protocol.DecisionRequest) (protocol.DecisionResponse, int, error)
}

func (p pdpRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var payload protocol.DecisionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, err
	}
	resp, status, err := p.onRequest(payload)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(resp)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    req,
	}, nil
}

type staticRoundTripper struct {
	status int
	body   string
}

func (s staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    req,
	}, nil
}

type errorRoundTripper struct {
	err error
}

func (e errorRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, e.err
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (r roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return r(req)
}
