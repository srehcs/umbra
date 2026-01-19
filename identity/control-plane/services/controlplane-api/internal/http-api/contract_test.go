package httpapi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openAPI struct {
	Paths      map[string]openAPIPath `yaml:"paths"`
	Components openAPIComponents      `yaml:"components"`
}

type openAPIComponents struct {
	Schemas map[string]*openAPISchema `yaml:"schemas"`
}

type openAPIPath struct {
	Get  *openAPIOperation `yaml:"get"`
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

func TestControlPlaneContractOpenAPI(t *testing.T) {
	spec := loadOpenAPI(t)

	assertSchemaExists(t, spec, "ReceiptList")
	assertSchemaExists(t, spec, "ReceiptIngestRequest")
	assertSchemaExists(t, spec, "ReceiptIngestResponse")
	assertSchemaExists(t, spec, "ReceiptVerifyResponse")
	assertSchemaExists(t, spec, "ReceiptExportResponse")
	assertSchemaExists(t, spec, "ActivePolicyResponse")

	errResp := derefSchema(t, spec, spec.Components.Schemas["ErrorResponse"])
	assertRequired(t, errResp.Required, "error", "request_id")
	assertErrorResponseMatchesProtocol(t, spec, errResp)

	receiptsPath := spec.Paths["/v1/receipts"]
	if receiptsPath.Get == nil || receiptsPath.Post == nil {
		t.Fatalf("expected /v1/receipts GET and POST operations")
	}
	assertResponseSchemaRef(t, receiptsPath.Get.Responses["200"], "ReceiptList")
	assertResponseSchemaRef(t, receiptsPath.Get.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, receiptsPath.Get.Responses["500"], "ErrorResponse")
	assertRequestSchemaRef(t, receiptsPath.Post.RequestBody, "ReceiptIngestRequest")
	assertResponseSchemaRef(t, receiptsPath.Post.Responses["200"], "ReceiptIngestResponse")
	assertResponseSchemaRef(t, receiptsPath.Post.Responses["201"], "ReceiptIngestResponse")
	assertResponseSchemaRef(t, receiptsPath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, receiptsPath.Post.Responses["409"], "ErrorResponse")
	assertResponseSchemaRef(t, receiptsPath.Post.Responses["500"], "ErrorResponse")

	verifyPath := spec.Paths["/v1/receipts/verify"]
	if verifyPath.Post == nil {
		t.Fatalf("expected /v1/receipts/verify POST operation")
	}
	assertResponseSchemaRef(t, verifyPath.Post.Responses["200"], "ReceiptVerifyResponse")
	assertResponseSchemaRef(t, verifyPath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, verifyPath.Post.Responses["500"], "ErrorResponse")

	exportPath := spec.Paths["/v1/receipts/export"]
	if exportPath.Get == nil {
		t.Fatalf("expected /v1/receipts/export GET operation")
	}
	assertResponseSchemaRef(t, exportPath.Get.Responses["200"], "ReceiptExportResponse")
	assertResponseSchemaRef(t, exportPath.Get.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, exportPath.Get.Responses["500"], "ErrorResponse")

	activePath := spec.Paths["/v1/policies/active"]
	if activePath.Get == nil {
		t.Fatalf("expected /v1/policies/active GET operation")
	}
	assertResponseSchemaRef(t, activePath.Get.Responses["200"], "ActivePolicyResponse")
	assertResponseSchemaRef(t, activePath.Get.Responses["404"], "ErrorResponse")
	assertResponseSchemaRef(t, activePath.Get.Responses["503"], "ErrorResponse")

	toolsPath := spec.Paths["/v1/tools"]
	if toolsPath.Get == nil || toolsPath.Post == nil {
		t.Fatalf("expected /v1/tools GET and POST operations")
	}
	assertResponseSchemaRef(t, toolsPath.Get.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, toolsPath.Get.Responses["500"], "ErrorResponse")
	assertResponseSchemaRef(t, toolsPath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, toolsPath.Post.Responses["500"], "ErrorResponse")

	policiesPath := spec.Paths["/v1/policies"]
	if policiesPath.Get == nil || policiesPath.Post == nil {
		t.Fatalf("expected /v1/policies GET and POST operations")
	}
	assertResponseSchemaRef(t, policiesPath.Get.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, policiesPath.Get.Responses["500"], "ErrorResponse")
	assertResponseSchemaRef(t, policiesPath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, policiesPath.Post.Responses["500"], "ErrorResponse")

	activatePath := spec.Paths["/v1/policies/activate"]
	if activatePath.Post == nil {
		t.Fatalf("expected /v1/policies/activate POST operation")
	}
	assertResponseSchemaRef(t, activatePath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, activatePath.Post.Responses["404"], "ErrorResponse")
	assertResponseSchemaRef(t, activatePath.Post.Responses["500"], "ErrorResponse")

	simulatePath := spec.Paths["/v1/policies/simulate"]
	if simulatePath.Post == nil {
		t.Fatalf("expected /v1/policies/simulate POST operation")
	}
	assertResponseSchemaRef(t, simulatePath.Post.Responses["400"], "ErrorResponse")
	assertResponseSchemaRef(t, simulatePath.Post.Responses["500"], "ErrorResponse")
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

func assertErrorResponseMatchesProtocol(t *testing.T, spec *openAPI, schema *openAPISchema) {
	t.Helper()
	assertProperties(t, schema, "error", "request_id", "decision_id", "trace_id")
	errObj := derefSchema(t, spec, schema.Properties["error"])
	assertRequired(t, errObj.Required, "code", "message")
	assertProperties(t, errObj, "code", "message", "details")
	if details := errObj.Properties["details"]; details == nil || details.Items == nil {
		t.Fatalf("expected error.details items")
	} else {
		detailsItem := derefSchema(t, spec, details.Items)
		assertProperties(t, detailsItem, "field", "message")
	}
}

func assertProperties(t *testing.T, schema *openAPISchema, fields ...string) {
	t.Helper()
	for _, field := range fields {
		if schema.Properties[field] == nil {
			t.Fatalf("missing schema property %s", field)
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

func assertRequestSchemaRef(t *testing.T, body *openAPIRequestBody, schemaName string) {
	t.Helper()
	if body == nil {
		t.Fatalf("missing request body")
	}
	mt, ok := body.Content["application/json"]
	if !ok || mt.Schema == nil || mt.Schema.Ref == "" {
		t.Fatalf("missing application/json schema for request body")
	}
	if mt.Schema.Ref != "#/components/schemas/"+schemaName {
		t.Fatalf("expected request schema %s, got %s", schemaName, mt.Schema.Ref)
	}
}

func assertSchemaExists(t *testing.T, spec *openAPI, name string) {
	t.Helper()
	if spec.Components.Schemas[name] == nil {
		t.Fatalf("missing schema: %s", name)
	}
}

func set(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}
