package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/testutil"
	"go.opentelemetry.io/otel/trace"
)

func TestPEPContractGoldenAllow(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tracer := trace.NewNoopTracerProvider().Tracer("test")
	fixtures := testutil.NewContractFixtures(
		"11111111-1111-1111-1111-111111111111",
		"",
		map[string]string{
			"x-umbra-tool-name": "demo.tool",
			"x-umbra-actor-id":  "user-1",
		},
	)
	testutil.RunContractSuite(t, testutil.ContractSuite{
		Name: "pep",
		Cases: []testutil.ContractCase{
			{
				Name:       "allow",
				Method:     http.MethodGet,
				Path:       "/tool/demo",
				RequestID:  "req-pep-allow-1",
				Fixtures:   fixtures,
				WantStatus: http.StatusOK,
				AssertBody: assertAllowGolden("pep_allow_response.json"),
				Handler:    pepHandler(tracer, logger, allowDecisionResponse()),
			},
			{
				Name:        "deny",
				Method:      http.MethodGet,
				Path:        "/tool/demo",
				RequestID:   "req-pep-1",
				Fixtures:    fixtures,
				WantStatus:  http.StatusForbidden,
				GoldenFile:  "pep_deny_error.json",
				Handler:     pepHandler(tracer, logger, denyDecisionResponse()),
				WantHeaders: map[string]string{"x-umbra-request-id": "req-pep-1"},
				WantError: &testutil.ErrorExpectation{
					Code:       "POLICY_DENIED",
					Message:    "test policy",
					RequestID:  "req-pep-1",
					DecisionID: "11111111-1111-1111-1111-111111111111",
				},
				Strict: true,
			},
			{
				Name:       "observe mode allows forward",
				Method:     http.MethodGet,
				Path:       "/tool/demo",
				RequestID:  "req-pep-observe-1",
				Fixtures:   fixtures,
				WantStatus: http.StatusOK,
				AssertBody: assertAllowGolden("pep_allow_response.json"),
				Handler:    pepObserveHandler(tracer, logger),
			},
		},
	})
}

func assertAllowGolden(name string) func(t *testing.T, body []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		var expected testutil.PEPAllowResponse
		testutil.LoadGoldenInto(t, name, &expected)
		var actual testutil.PEPAllowResponse
		if err := json.Unmarshal(body, &actual); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		testutil.AssertPEPAllowResponse(t, expected, actual)
	}
}

func pepHandler(tracer trace.Tracer, logger *slog.Logger, response protocol.DecisionResponse) http.Handler {
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				response.RequestID = req.Trace.RequestID
				response.TraceID = req.Trace.TraceID
				response.SpanID = req.Trace.SpanID
				return response
			},
		}},
	}

	upstreamURL, _ := url.Parse("http://upstream.invalid")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	proxy.Transport = staticRoundTripper{status: http.StatusOK, body: `{"status":"ok"}`}

	return handleToolProxy(tracer, logger, nil, pdp, proxy, "enforce")
}

func allowDecisionResponse() protocol.DecisionResponse {
	return protocol.DecisionResponse{
		Decision:      "allow",
		DecisionID:    "11111111-1111-1111-1111-111111111111",
		PolicyVersion: 1,
		PolicyHash:    "policy-hash",
		Reason:        "test policy",
	}
}

func denyDecisionResponse() protocol.DecisionResponse {
	return protocol.DecisionResponse{
		Decision:   "deny",
		DecisionID: "11111111-1111-1111-1111-111111111111",
		Reason:     "test policy",
	}
}

func pepObserveHandler(tracer trace.Tracer, logger *slog.Logger) http.Handler {
	pdp := &PDPClient{
		BaseURL: "http://pdp.invalid",
		Client: &http.Client{Transport: captureRoundTripper{
			onRequest: func(req protocol.DecisionRequest) protocol.DecisionResponse {
				return protocol.DecisionResponse{
					Decision:   "deny",
					DecisionID: "11111111-1111-1111-1111-111111111111",
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

	return handleToolProxy(tracer, logger, nil, pdp, proxy, "observe")
}
