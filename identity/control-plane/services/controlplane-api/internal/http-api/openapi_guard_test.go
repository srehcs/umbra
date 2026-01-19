package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type openAPIBaseline struct {
	Schemas   map[string]schemaBaseline                         `json:"schemas"`
	Responses map[string]map[string]map[string]responseBaseline `json:"responses"`
}

type schemaBaseline struct {
	Required []string `json:"required"`
}

type responseBaseline struct {
	SchemaRef    string   `json:"schema_ref"`
	ContentTypes []string `json:"content_types"`
}

func TestOpenAPICompatibilityGuard(t *testing.T) {
	spec := loadOpenAPI(t)
	baseline := loadOpenAPIBaseline(t)

	for name, schema := range baseline.Schemas {
		actual := spec.Components.Schemas[name]
		if actual == nil {
			t.Fatalf("missing schema %s", name)
		}
		for _, field := range schema.Required {
			if !contains(actual.Required, field) {
				t.Fatalf("schema %s missing required field %s", name, field)
			}
		}
	}

	for path, methods := range baseline.Responses {
		pathSpec, ok := spec.Paths[path]
		if !ok {
			t.Fatalf("missing path %s", path)
		}
		for method, statuses := range methods {
			op := operationForMethod(pathSpec, method)
			if op == nil {
				t.Fatalf("missing %s %s operation", strings.ToUpper(method), path)
			}
			for status, expected := range statuses {
				resp, ok := op.Responses[status]
				if !ok {
					t.Fatalf("missing response %s for %s %s", status, strings.ToUpper(method), path)
				}
				for _, ct := range expected.ContentTypes {
					mt, ok := resp.Content[ct]
					if !ok || mt.Schema == nil {
						t.Fatalf("missing content type %s for %s %s %s", ct, strings.ToUpper(method), path, status)
					}
					if expected.SchemaRef != "" && mt.Schema.Ref != expected.SchemaRef {
						t.Fatalf("schema ref drift for %s %s %s: expected %s got %s", strings.ToUpper(method), path, status, expected.SchemaRef, mt.Schema.Ref)
					}
				}
			}
		}
	}
}

func operationForMethod(pathSpec openAPIPath, method string) *openAPIOperation {
	switch strings.ToLower(method) {
	case "get":
		return pathSpec.Get
	case "post":
		return pathSpec.Post
	default:
		return nil
	}
}

func loadOpenAPIBaseline(t *testing.T) openAPIBaseline {
	t.Helper()
	data, err := os.ReadFile(openAPIBaselinePath(t))
	if err != nil {
		t.Fatalf("read openapi baseline: %v", err)
	}
	var out openAPIBaseline
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse openapi baseline: %v", err)
	}
	return out
}

func openAPIBaselinePath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	return filepath.Join(root, "docs", "test_vectors", "contracts", "openapi_baseline.json")
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
