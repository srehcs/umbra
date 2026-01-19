package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

type ContractCase struct {
	Name        string
	Method      string
	Path        string
	Headers     map[string]string
	Body        []byte
	WantStatus  int
	WantHeaders map[string]string
	GoldenFile  string
	Handler     http.Handler
	WantError   *ErrorExpectation
	Strict      bool
	RequestID   string
	TenantID    string
	AssertBody  func(t *testing.T, body []byte)
	Fixtures    *ContractFixtures
}

type ContractSuite struct {
	Name    string
	Handler http.Handler
	Cases   []ContractCase
}

type ErrorExpectation struct {
	Code       string
	Message    string
	RequestID  string
	DecisionID string
	TraceID    string
}

func RunContractSuite(t *testing.T, suite ContractSuite) {
	t.Helper()
	if suite.Handler == nil && len(suite.Cases) > 0 {
		for _, tc := range suite.Cases {
			if tc.Handler == nil {
				t.Fatalf("contract suite %s missing handler for case %s", suite.Name, tc.Name)
			}
		}
	}
	for _, tc := range suite.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			handler := tc.Handler
			if handler == nil {
				handler = suite.Handler
			}
			req := httptest.NewRequest(tc.Method, tc.Path, bytes.NewReader(tc.Body))
			applyContractFixtures(req, tc)
			for k, v := range tc.Headers {
				req.Header.Set(k, v)
			}
			if len(tc.Body) > 0 && req.Header.Get("content-type") == "" {
				req.Header.Set("content-type", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			AssertStatus(t, rec, tc.WantStatus)
			bodyBytes := rec.Body.Bytes()
			AssertHeaders(t, rec, tc.WantHeaders)
			if tc.WantError != nil {
				var errResp protocol.ErrorResponse
				if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				AssertErrorEnvelope(t, errResp, tc.WantError.Code, tc.WantError.Message, tc.WantError.RequestID, tc.WantError.DecisionID, tc.WantError.TraceID)
			}
			if tc.AssertBody != nil {
				tc.AssertBody(t, bodyBytes)
			} else if tc.GoldenFile != "" {
				AssertJSONBody(t, bodyBytes, tc.GoldenFile, tc.Strict)
			}
		})
	}
}

type ContractFixtures struct {
	TenantID  string
	RequestID string
	Headers   map[string]string
}

func NewContractFixtures(tenantID, requestID string, headers map[string]string) *ContractFixtures {
	merged := make(map[string]string, len(headers))
	for k, v := range headers {
		merged[k] = v
	}
	return &ContractFixtures{
		TenantID:  tenantID,
		RequestID: requestID,
		Headers:   merged,
	}
}

func applyContractFixtures(req *http.Request, tc ContractCase) {
	if tc.Fixtures != nil {
		if tc.Fixtures.RequestID != "" {
			req.Header.Set("x-umbra-request-id", tc.Fixtures.RequestID)
		}
		if tc.Fixtures.TenantID != "" {
			req.Header.Set("x-umbra-tenant-id", tc.Fixtures.TenantID)
		}
		for k, v := range tc.Fixtures.Headers {
			req.Header.Set(k, v)
		}
	}
	if tc.RequestID != "" {
		req.Header.Set("x-umbra-request-id", tc.RequestID)
	}
	if tc.TenantID != "" {
		req.Header.Set("x-umbra-tenant-id", tc.TenantID)
	}
}

func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d", status, rec.Code)
	}
}

func AssertHeaders(t *testing.T, rec *httptest.ResponseRecorder, headers map[string]string) {
	t.Helper()
	for key, expected := range headers {
		assertHeader(t, rec.Header().Get(key), expected, key)
	}
}

func AssertJSONBody(t *testing.T, body []byte, goldenFile string, strict bool) {
	t.Helper()
	if goldenFile == "" {
		return
	}
	goldenBytes := LoadGoldenBytes(t, goldenFile)
	AssertJSONBytesMatch(t, goldenBytes, body, strict)
}

func assertHeader(t *testing.T, actual, expected, name string) {
	t.Helper()
	switch expected {
	case GoldenAny:
		if actual == "" {
			t.Fatalf("expected header %s to be set", name)
		}
	case GoldenNonEmpty:
		if actual == "" {
			t.Fatalf("expected header %s to be non-empty", name)
		}
	default:
		if actual != expected {
			t.Fatalf("expected header %s=%q, got %q", name, expected, actual)
		}
	}
}
