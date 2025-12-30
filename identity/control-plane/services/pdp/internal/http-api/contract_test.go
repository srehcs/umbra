package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"log/slog"

	"gopkg.in/yaml.v3"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

type openAPI struct {
	Paths      map[string]openAPIPath `yaml:"paths"`
	Components openAPIComponents      `yaml:"components"`
}

type openAPIComponents struct {
	Schemas map[string]*openAPISchema `yaml:"schemas"`
}

type openAPIPath struct {
	Post *openAPIOperation `yaml:"post"`
}

type openAPIOperation struct {
	Responses   map[string]openAPIResponse `yaml:"responses"`
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIResponse struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIMediaType struct {
	Schema *openAPISchema `yaml:"schema"`
}

type openAPISchema struct {
	Ref        string                    `yaml:"$ref"`
	Type       string                    `yaml:"type"`
	Properties map[string]*openAPISchema `yaml:"properties"`
	Required   []string                  `yaml:"required"`
	Enum       []string                  `yaml:"enum"`
	Items      *openAPISchema            `yaml:"items"`
	Nullable   bool                      `yaml:"nullable"`
}

func TestDecisionContractOpenAPI(t *testing.T) {
	spec := loadOpenAPI(t)
	decisionReq := derefSchema(t, spec, spec.Components.Schemas["DecisionRequest"])
	assertRequired(t, decisionReq.Required, "tenant", "actor", "tool")

	tenant := derefSchema(t, spec, decisionReq.Properties["tenant"])
	assertRequired(t, tenant.Required, "tenant_id")

	actor := derefSchema(t, spec, decisionReq.Properties["actor"])
	assertRequired(t, actor.Required, "type", "id")

	tool := derefSchema(t, spec, decisionReq.Properties["tool"])
	assertRequired(t, tool.Required, "name", "method", "endpoint")

	decisionResp := derefSchema(t, spec, spec.Components.Schemas["DecisionResponse"])
	assertRequired(t, decisionResp.Required, "decision", "decision_id")
	decisionEnum := set(decisionResp.Properties["decision"].Enum)
	assertInSet(t, decisionEnum, "allow", "deny")

	errResp := derefSchema(t, spec, spec.Components.Schemas["ErrorResponse"])
	codeEnum := set(errResp.Properties["error_code"].Enum)
	assertInSet(t, codeEnum, "POLICY_DENIED", "POLICY_INVALID", "POLICY_UNAVAILABLE")

	op := spec.Paths["/v1/decision"].Post
	if op == nil {
		t.Fatalf("missing /v1/decision POST operation")
	}
	assertResponseSchemaRef(t, op.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, op.Responses["500"], "ErrorResponse")
	assertResponseSchemaRef(t, op.Responses["503"], "ErrorResponse")
}

func TestDecisionContractRuntime(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(ioDiscard{}, nil))
	mux := http.NewServeMux()
	registerV0(mux, logger)

	req := protocol.DecisionRequest{
		Tenant: protocol.TenantContext{TenantID: "00000000-0000-0000-0000-000000000001"},
		Actor:  protocol.Actor{Type: "human", ID: "user-1", Roles: []string{"developer"}},
		Tool:   protocol.Tool{Name: "demo.tool", Method: "GET", Endpoint: "/demo"},
		Trace: &protocol.TraceContext{
			RequestID: "req-123",
			TraceID:   "trace-123",
			SpanID:    "span-123",
		},
	}
	body, _ := json.Marshal(req)
	reqRec := httptest.NewRecorder()
	reqHTTP := httptest.NewRequest(http.MethodPost, "/v1/decision", bytes.NewReader(body))
	reqHTTP.Header.Set("content-type", "application/json")
	mux.ServeHTTP(reqRec, reqHTTP)

	if reqRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", reqRec.Code)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(reqRec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	assertFieldPresent(t, out, "decision")
	assertFieldPresent(t, out, "decision_id")
	if out["request_id"] != "req-123" {
		t.Fatalf("expected request_id to round-trip, got %v", out["request_id"])
	}

	invalid := protocol.DecisionRequest{
		Tenant: protocol.TenantContext{TenantID: "not-a-uuid"},
		Actor:  protocol.Actor{Type: "human", ID: "user-1"},
		Tool:   protocol.Tool{Name: "demo.tool", Method: "GET", Endpoint: "/demo"},
	}
	body, _ = json.Marshal(invalid)
	errRec := httptest.NewRecorder()
	errReq := httptest.NewRequest(http.MethodPost, "/v1/decision", bytes.NewReader(body))
	errReq.Header.Set("content-type", "application/json")
	mux.ServeHTTP(errRec, errReq)

	if errRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", errRec.Code)
	}
	var errOut map[string]interface{}
	if err := json.NewDecoder(errRec.Body).Decode(&errOut); err != nil {
		t.Fatalf("decode error response failed: %v", err)
	}
	if errOut["error_code"] != "POLICY_INVALID" {
		t.Fatalf("expected error_code POLICY_INVALID, got %v", errOut["error_code"])
	}
}

func loadOpenAPI(t *testing.T) *openAPI {
	t.Helper()
	data, err := os.ReadFile(openapiPath(t))
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	var spec openAPI
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	return &spec
}

func openapiPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	return filepath.Join(root, "docs", "api", "openapi.yaml")
}

func derefSchema(t *testing.T, spec *openAPI, schema *openAPISchema) *openAPISchema {
	t.Helper()
	if schema == nil {
		t.Fatalf("schema is nil")
	}
	if schema.Ref == "" {
		return schema
	}
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(schema.Ref, prefix) {
		t.Fatalf("unsupported ref: %s", schema.Ref)
	}
	name := strings.TrimPrefix(schema.Ref, prefix)
	ref := spec.Components.Schemas[name]
	if ref == nil {
		t.Fatalf("missing schema ref: %s", name)
	}
	return ref
}

func assertRequired(t *testing.T, required []string, fields ...string) {
	t.Helper()
	req := set(required)
	for _, field := range fields {
		if _, ok := req[field]; !ok {
			t.Fatalf("missing required field: %s", field)
		}
	}
}

func assertInSet(t *testing.T, s map[string]struct{}, values ...string) {
	t.Helper()
	for _, v := range values {
		if _, ok := s[v]; !ok {
			t.Fatalf("missing enum value: %s", v)
		}
	}
}

func assertResponseSchemaRef(t *testing.T, resp openAPIResponse, schemaName string) {
	t.Helper()
	mt, ok := resp.Content["application/json"]
	if !ok || mt.Schema == nil || mt.Schema.Ref == "" {
		t.Fatalf("missing application/json schema for response")
	}
	if mt.Schema.Ref != "#/components/schemas/"+schemaName {
		t.Fatalf("expected response schema %s, got %s", schemaName, mt.Schema.Ref)
	}
}

func assertFieldPresent(t *testing.T, payload map[string]interface{}, field string) {
	t.Helper()
	if _, ok := payload[field]; !ok {
		t.Fatalf("missing field %s in response", field)
	}
}

func set(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
