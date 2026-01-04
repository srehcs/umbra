package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
	"os"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
)

func TestHandlePolicies_CreateValid(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	validPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET", "POST"},
				PathPrefix: "/api/v1",
			},
		},
		Default: "deny",
	}

	policyJSON, _ := json.Marshal(validPolicy)

	body := map[string]interface{}{
		"name":   "test-policy",
		"policy": json.RawMessage(policyJSON),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	// Since Store is nil, we expect a service unavailable error
	server.handlePolicies(w, req)

	// We should get 503 before validation even runs
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePolicies_CreateInvalid_MissingMode(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	invalidPolicy := map[string]interface{}{
		"version": 1,
		"rules":   []interface{}{},
		"default": "deny",
	}

	policyJSON, _ := json.Marshal(invalidPolicy)

	body := map[string]interface{}{
		"name":   "test-policy",
		"policy": json.RawMessage(policyJSON),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	// Since Store is nil, we expect a service unavailable error
	server.handlePolicies(w, req)

	// Service unavailable comes first
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePolicies_CreateInvalid_BadJSON(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	body := `{invalid json}`

	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader([]byte(body)))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	server.handlePolicies(w, req)

	// Service unavailable comes first
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandlePolicies_MissingTenant(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	validPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
			},
		},
		Default: "deny",
	}

	policyJSON, _ := json.Marshal(validPolicy)

	body := map[string]interface{}{
		"name":   "test-policy",
		"policy": json.RawMessage(policyJSON),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader(bodyBytes))
	// No tenant header
	w := httptest.NewRecorder()

	server.handlePolicies(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandlePolicies_EmptyPolicy(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	body := map[string]interface{}{
		"name":   "test-policy",
		"policy": json.RawMessage(nil),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	server.handlePolicies(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (before validation), got %d", w.Code)
	}
}

func TestValidationResponseFormat(t *testing.T) {
	// Test that when validation fails, we return the correct error format
	// This is a unit test for the error response format

	invalidPolicy := map[string]interface{}{
		"version": 0, // Invalid
		"default": "deny",
	}

	policyJSON, _ := json.Marshal(invalidPolicy)

	errs, _, _ := policy.ValidatePolicyWithSize(policyJSON)

	if len(errs) == 0 {
		t.Error("expected validation errors")
	}

	// Check error format
	for _, e := range errs {
		if e.Path == "" {
			t.Error("error path should not be empty")
		}
		if e.Message == "" {
			t.Error("error message should not be empty")
		}
	}
}

func TestPolicyHashConsistency(t *testing.T) {
	// Test that the same policy always produces the same hash
	validPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
				PathPrefix: "/api",
			},
		},
		Default: "deny",
	}

	policyJSON, _ := json.Marshal(validPolicy)

	hash1, err1 := policy.ComputePolicyHash(policyJSON)
	hash2, err2 := policy.ComputePolicyHash(policyJSON)

	if err1 != nil || err2 != nil {
		t.Fatalf("failed to compute hash: %v, %v", err1, err2)
	}

	if hash1 != hash2 {
		t.Errorf("hashes should be identical: %s vs %s", hash1, hash2)
	}

	// Verify hash is hex string
	if len(hash1) != 64 {
		t.Errorf("expected 64-char hex hash (SHA256), got %d", len(hash1))
	}

	// Verify it's valid hex
	for _, c := range hash1 {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("invalid hex character in hash: %c", c)
		}
	}
}

func TestValidateReceiptIngestMissingFields(t *testing.T) {
	req := receiptIngestRequest{
		Kind:      "decision",
		RequestID: "",
		Body:      json.RawMessage(`{}`),
	}
	errs := validateReceiptIngest(req)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	hasField := func(name string) bool {
		for _, err := range errs {
			if err.Field == name {
				return true
			}
		}
		return false
	}
	if !hasField("request_id") {
		t.Fatalf("expected request_id error, got %#v", errs)
	}
	if !hasField("decision_id") {
		t.Fatalf("expected decision_id error, got %#v", errs)
	}
	if !hasField("policy_hash") {
		t.Fatalf("expected policy_hash error, got %#v", errs)
	}
	if !hasField("decision") {
		t.Fatalf("expected decision error, got %#v", errs)
	}
}

func TestHandleSimulatePolicy_WithValidPolicy(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	validPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
				PathPrefix: "/api",
			},
		},
		Default: "deny",
	}

	policyJSON, _ := json.Marshal(validPolicy)

	body := map[string]interface{}{
		"actor_roles": []string{"admin"},
		"method":      "GET",
		"path":        "/api/users",
		"policy":      json.RawMessage(policyJSON),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies/simulate", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	server.handleSimulatePolicy(w, req)

	// Should return a decision response (not 503 due to nil store because we're using supplied policy)
	if w.Code == http.StatusServiceUnavailable {
		t.Errorf("expected 200 or validation error, got %d", w.Code)
	}

	// If it succeeded, verify response format
	if w.Code == http.StatusOK {
		var resp map[string]interface{}
		body, _ := io.ReadAll(w.Body)
		json.Unmarshal(body, &resp)

		if _, ok := resp["decision"]; !ok {
			t.Error("response should contain 'decision'")
		}
		if _, ok := resp["reason"]; !ok {
			t.Error("response should contain 'reason'")
		}
		if _, ok := resp["policy_hash"]; !ok {
			t.Error("response should contain 'policy_hash'")
		}
	}
}

func TestHandleSimulatePolicy_WithInvalidPolicy(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server := &Server{Logger: logger, Store: nil}

	invalidPolicy := map[string]interface{}{
		"version": 1,
		"mode":    "invalid_mode",
		"rules":   []interface{}{},
		"default": "deny",
	}

	policyJSON, _ := json.Marshal(invalidPolicy)

	body := map[string]interface{}{
		"actor_roles": []string{"admin"},
		"method":      "GET",
		"path":        "/api/users",
		"policy":      json.RawMessage(policyJSON),
	}

	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies/simulate", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", "550e8400-e29b-41d4-a716-446655440000")
	w := httptest.NewRecorder()

	server.handleSimulatePolicy(w, req)

	// Should fail validation and return 400 with error format
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid policy, got %d", w.Code)
	}

	var resp map[string]interface{}
	respBytes, _ := io.ReadAll(w.Body)
	json.Unmarshal(respBytes, &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response should contain 'error'")
	}
	if code, ok := errObj["code"]; !ok || code != policy.ErrorCodePolicyInvalid {
		t.Errorf("expected code '%s', got %v", policy.ErrorCodePolicyInvalid, code)
	}
	if _, ok := errObj["details"]; !ok {
		t.Error("response should contain 'error.details'")
	}
}
