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
			// Create a mock upstream server
			upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer upstreamServer.Close()

			// Create a mock PDP server
			pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := map[string]interface{}{
					"decision_id": uuid.NewString(),
					"decision":    tt.decision,
					"reason":      "test policy",
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer pdpServer.Close()

			// Create the router with PEP_MODE
			t.Setenv("PEP_MODE", tt.pepMode)
			t.Setenv("PDP_URL", pdpServer.URL)
			t.Setenv("UPSTREAM_URL", upstreamServer.URL)

			mux := http.NewServeMux()
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

			// Manually register V0
			tracer := trace.NewNoopTracerProvider().Tracer("test")
			pdp := &PDPClient{
				BaseURL: pdpServer.URL,
				Client:  &http.Client{},
			}
			upstreamURL, _ := url.Parse(upstreamServer.URL)
			proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

			mux.Handle("/tool/", handleToolProxy(tracer, logger, nil, pdp, proxy, tt.pepMode))

			server := httptest.NewServer(mux)
			defer server.Close()

			// Make a request
			req, _ := http.NewRequest(http.MethodGet, server.URL+"/tool/test", nil)
			req.Header.Set("x-umbra-tenant-id", uuid.NewString())
			req.Header.Set("x-umbra-actor-id", "test-user")
			req.Header.Set("x-umbra-tool-name", "test-tool")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// For enforce+deny, check response is POLICY_DENIED
			if tt.pepMode == "enforce" && tt.decision == "deny" {
				var errResp blockedResponse
				json.NewDecoder(resp.Body).Decode(&errResp)
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
}

type captureStore struct {
	captured capturedInvocation
}

func (s *captureStore) LastInvocationHash(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}

func (s *captureStore) InsertInvocationReceipt(_ context.Context, _ uuid.UUID, decisionID *uuid.UUID, requestID string, _ string, _ string, _ string, _ string, _ *int, _ int, _ json.RawMessage, _ string, _ string, traceID string, spanID string) error {
	s.captured = capturedInvocation{
		requestID:  requestID,
		decisionID: decisionID,
		traceID:    traceID,
		spanID:     spanID,
	}
	return nil
}

func TestInvocationCorrelationIDs(t *testing.T) {
	var pdpRequestID string
	var decisionID string

	pdpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.DecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("pdp request decode failed: %v", err)
			return
		}
		if req.Trace == nil || req.Trace.RequestID == "" {
			t.Errorf("expected request_id in pdp request")
		}
		pdpRequestID = req.Trace.RequestID
		decisionID = uuid.NewString()
		resp := protocol.DecisionResponse{
			Decision:   "deny",
			DecisionID: decisionID,
			Reason:     "test policy",
			RequestID:  req.Trace.RequestID,
			TraceID:    req.Trace.TraceID,
			SpanID:     req.Trace.SpanID,
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer pdpServer.Close()

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstreamServer.Close()

	store := &captureStore{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	pdp := &PDPClient{
		BaseURL: pdpServer.URL,
		Client:  &http.Client{},
	}
	upstreamURL, _ := url.Parse(upstreamServer.URL)
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	mux := http.NewServeMux()
	mux.Handle("/tool/", handleToolProxy(tracer, logger, store, pdp, proxy, "enforce"))

	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/tool/test", nil)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	req.Header.Set("x-umbra-actor-id", "test-user")
	req.Header.Set("x-umbra-tool-name", "test-tool")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var errResp blockedResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
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
