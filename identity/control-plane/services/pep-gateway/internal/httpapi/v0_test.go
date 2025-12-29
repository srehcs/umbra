package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"log/slog"
	"net/http/httputil"
	"net/url"
	"os"
)

// TestObserveVsEnforceMode tests that PEP_MODE correctly controls behavior for DENY decisions
func TestObserveVsEnforceMode(t *testing.T) {
	tests := []struct {
		name           string
		pepMode        string
		decision       string // "allow" or "deny"
		expectedStatus int
		expectedBody   string
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
			tracer := noopTracer{}
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

// noopTracer is a minimal tracer implementation for testing
type noopTracer struct{}

func (t noopTracer) Start(ctx context.Context, spanName string, opts ...interface{}) (context.Context, interface{}) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (s noopSpan) End(...interface{}) {}

func (s noopSpan) SetAttributes(...interface{}) {}
