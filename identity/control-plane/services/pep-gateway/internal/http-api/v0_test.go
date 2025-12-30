package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel/trace"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

// TestObserveVsEnforceMode tests that PEP_MODE correctly controls behavior for DENY decisions
func TestObserveVsEnforceMode(t *testing.T) {
	tests := []struct {
		name            string
		pepMode         string
		decision        string // "allow" or "deny"
		expectedStatus  int
		expectedBody    string
		expectedOutcome string
	}{
		{
			name:            "observe mode with deny forwards request",
			pepMode:         "observe",
			decision:        "deny",
			expectedStatus:  http.StatusOK,
			expectedOutcome: "forwarded",
		},
		{
			name:            "enforce mode with deny blocks request",
			pepMode:         "enforce",
			decision:        "deny",
			expectedStatus:  http.StatusForbidden,
			expectedOutcome: "blocked",
		},
		{
			name:            "observe mode with allow forwards request",
			pepMode:         "observe",
			decision:        "allow",
			expectedStatus:  http.StatusOK,
			expectedOutcome: "forwarded",
		},
		{
			name:            "enforce mode with allow forwards request",
			pepMode:         "enforce",
			decision:        "allow",
			expectedStatus:  http.StatusOK,
			expectedOutcome: "forwarded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PEP_MODE", tt.pepMode)
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

			// Manually register V0
			tracer := trace.NewNoopTracerProvider().Tracer("test")
			pdp := &PDPClient{
				BaseURL: "http://pdp.invalid",
				Client:  &http.Client{Transport: decisionRoundTripper{decision: tt.decision}},
			}
			upstreamURL, _ := url.Parse("http://upstream.invalid")
			proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
			proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}

			// Make a request
			req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
			req.Header.Set("x-umbra-tenant-id", uuid.NewString())
			req.Header.Set("x-umbra-actor-id", "test-user")
			req.Header.Set("x-umbra-tool-name", "test-tool")

			rec := httptest.NewRecorder()
			handleToolProxy(tracer, logger, nil, pdp, proxy, tt.pepMode).ServeHTTP(rec, req)

			// Check status code
			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// For enforce+deny, check response is POLICY_DENIED
			if tt.pepMode == "enforce" && tt.decision == "deny" {
				var errResp blockedResponse
				json.NewDecoder(rec.Body).Decode(&errResp)
				if errResp.ErrorCode != "POLICY_DENIED" {
					t.Errorf("expected error code POLICY_DENIED, got %s", errResp.ErrorCode)
				}
				if errResp.RequestID == "" {
					t.Error("expected request_id in error response")
				}
				if errResp.DecisionID == "" {
					t.Error("expected decision_id in error response")
				}
			}
		})
	}
}

type capturedInvocation struct {
	requestID  string
	decisionID *uuid.UUID
	traceID    string
	spanID     string
	body       json.RawMessage
}

type captureStore struct {
	captured capturedInvocation
}

func (s *captureStore) LastInvocationHash(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}

func (s *captureStore) InsertInvocationReceipt(_ context.Context, _ uuid.UUID, decisionID *uuid.UUID, requestID string, _ string, _ string, _ string, _ string, _ *int, _ int, body json.RawMessage, _ string, _ string, traceID string, spanID string) error {
	s.captured = capturedInvocation{
		requestID:  requestID,
		decisionID: decisionID,
		traceID:    traceID,
		spanID:     spanID,
		body:       append(json.RawMessage(nil), body...),
	}
	return nil
}

func TestInvocationCorrelationIDs(t *testing.T) {
	var pdpRequestID string
	var decisionID string

	store := &captureStore{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				if req.Trace == nil || req.Trace.RequestID == "" {
					t.Errorf("expected request_id in pdp request")
				}
				pdpRequestID = req.Trace.RequestID
				decisionID = uuid.NewString()
				return protocol.DecisionResponse{
					Decision:   "deny",
					DecisionID: decisionID,
					Reason:     "test policy",
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

	mux := http.NewServeMux()
	mux.Handle("/tool/", handleToolProxy(tracer, logger, store, pdp, proxy, "enforce"))

	req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	req.Header.Set("x-umbra-actor-id", "test-user")
	req.Header.Set("x-umbra-tool-name", "test-tool")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var errResp blockedResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if errResp.RequestID == "" {
		t.Fatal("expected request_id in response")
	}
	if errResp.DecisionID == "" {
		t.Fatal("expected decision_id in response")
	}
	if pdpRequestID == "" {
		t.Fatal("expected request_id in pdp request")
	}
	if pdpRequestID != errResp.RequestID {
		t.Fatalf("pdp request_id mismatch: %s vs %s", pdpRequestID, errResp.RequestID)
	}
	if decisionID != errResp.DecisionID {
		t.Fatalf("decision_id mismatch: %s vs %s", decisionID, errResp.DecisionID)
	}
	if store.captured.requestID != errResp.RequestID {
		t.Fatalf("stored request_id mismatch: %s vs %s", store.captured.requestID, errResp.RequestID)
	}
	if store.captured.decisionID == nil || store.captured.decisionID.String() != errResp.DecisionID {
		t.Fatalf("stored decision_id mismatch: %v vs %s", store.captured.decisionID, errResp.DecisionID)
	}
}

func TestPDPUnavailableObserveVsEnforce(t *testing.T) {
	tests := []struct {
		name           string
		pepMode        string
		expectedStatus int
		expectedCode   string
	}{
		{name: "observe forwards on pdp unavailable", pepMode: "observe", expectedStatus: http.StatusOK},
		{name: "enforce blocks on pdp unavailable", pepMode: "enforce", expectedStatus: http.StatusServiceUnavailable, expectedCode: "POLICY_UNAVAILABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			tracer := trace.NewNoopTracerProvider().Tracer("test")
			store := &captureStore{}
			pdp := &PDPClient{
				BaseURL: "http://pdp.invalid",
				Client:  &http.Client{Transport: errorRoundTripper{err: context.DeadlineExceeded}},
			}
			upstreamURL, _ := url.Parse("http://upstream.invalid")
			proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
			proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}

			req := httptest.NewRequest(http.MethodGet, "/tool/test", nil)
			req.Header.Set("x-umbra-tenant-id", uuid.NewString())
			req.Header.Set("x-umbra-actor-id", "test-user")
			req.Header.Set("x-umbra-tool-name", "test-tool")

			rec := httptest.NewRecorder()
			handleToolProxy(tracer, logger, store, pdp, proxy, tt.pepMode).ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if len(store.captured.body) == 0 {
				t.Fatalf("expected invocation receipt to be captured")
			}
			var receipt map[string]interface{}
			if err := json.Unmarshal(store.captured.body, &receipt); err != nil {
				t.Fatalf("decode receipt failed: %v", err)
			}
			if receipt["pdp.status"] == "" {
				t.Fatalf("expected pdp.status in receipt")
			}
			if receipt["enforcement.outcome"] == "" {
				t.Fatalf("expected enforcement.outcome in receipt")
			}
			if tt.expectedCode != "" {
				var errResp blockedResponse
				if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
					t.Fatalf("decode response failed: %v", err)
				}
				if errResp.ErrorCode != tt.expectedCode {
					t.Fatalf("expected error code %s, got %s", tt.expectedCode, errResp.ErrorCode)
				}
			}
		})
	}
}

type staticRoundTripper struct {
	status int
	body   string
}

func (s staticRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	res := &http.Response{
		StatusCode: s.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Request:    req,
	}
	return res, nil
}

type decisionRoundTripper struct {
	decision string
}

func (d decisionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := map[string]interface{}{
		"decision_id": uuid.NewString(),
		"decision":    d.decision,
		"reason":      "test policy",
	}
	body, _ := json.Marshal(resp)
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    req,
	}
	return res, nil
}

type errorRoundTripper struct {
	err error
}

func (e errorRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, e.err
}

type captureRoundTripper struct {
	onRequest func(req protocol.DecisionRequest) protocol.DecisionResponse
}

func (c captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var payload protocol.DecisionRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, err
	}
	resp := c.onRequest(payload)
	body, _ := json.Marshal(resp)
	res := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    req,
	}
	return res, nil
}
