package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidatePolicy_ValidPolicy(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET", "POST"},
				PathPrefix: "/api/v1",
			},
		},
		Default: "deny",
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("failed to marshal policy: %v", err)
	}

	errs := ValidatePolicy(data)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestValidatePolicy_EmptyPolicy(t *testing.T) {
	errs := ValidatePolicy([]byte{})
	if len(errs) == 0 {
		t.Error("expected validation error for empty policy")
	}
	if errs[0].Path != "policy" {
		t.Errorf("expected error path 'policy', got '%s'", errs[0].Path)
	}
}

func TestValidatePolicy_InvalidJSON(t *testing.T) {
	errs := ValidatePolicy([]byte(`{"invalid": json}`))
	if len(errs) == 0 {
		t.Error("expected validation error for invalid JSON")
	}
	if errs[0].Path != "policy" {
		t.Errorf("expected error path 'policy', got '%s'", errs[0].Path)
	}
}

func TestValidatePolicy_MissingVersion(t *testing.T) {
	policy := Policy{
		Mode:    "abac_v0",
		Rules:   []Rule{},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing version")
	}
}

func TestValidatePolicy_MissingMode(t *testing.T) {
	policy := Policy{
		Version: 1,
		Rules:   []Rule{},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "mode" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing mode")
	}
}

func TestValidatePolicy_InvalidMode(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "invalid_mode",
		Rules:   []Rule{},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "mode" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for invalid mode")
	}
}

func TestValidatePolicy_MissingDefault(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules:   []Rule{},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing default")
	}
}

func TestValidatePolicy_InvalidDefault(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules:   []Rule{},
		Default: "maybe",
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "default" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for invalid default")
	}
}

func TestValidatePolicy_MissingRules(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for missing rules")
	}
}

func TestValidatePolicy_ExceedsMaxSize(t *testing.T) {
	// Create a policy larger than max
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:     "allow",
				PathPrefix: strings.Repeat("x", PolicyMaxBytes),
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	if len(errs) == 0 {
		t.Error("expected validation error for size limit")
	}
}

func TestValidatePolicy_InvalidEffect(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect: "maybe",
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules[0].effect" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for invalid effect")
	}
}

func TestValidatePolicy_InvalidMethod(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:     "allow",
				MethodsAny: []string{"INVALID"},
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules[0].methods_any[0]" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for invalid HTTP method")
	}
}

func TestValidatePolicy_EmptyMethod(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:     "allow",
				MethodsAny: []string{""},
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules[0].methods_any[0]" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for empty method")
	}
}

func TestValidatePolicy_EmptyRole(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect:   "allow",
				RolesAny: []string{""},
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules[0].roles_any[0]" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for empty role")
	}
}

func TestValidatePolicy_RuleWithNoConditions(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Default: "deny",
		Rules: []Rule{
			{
				Effect: "allow",
			},
		},
	}

	data, _ := json.Marshal(policy)
	errs := ValidatePolicy(data)

	found := false
	for _, e := range errs {
		if e.Path == "rules[0]" && string(rune(e.Message[0])) != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected validation error for rule with no conditions")
	}
}

func TestComputePolicyHash_Deterministic(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
				PathPrefix: "/api",
			},
		},
		Default: "deny",
	}

	data1, _ := json.Marshal(policy)
	data2, _ := json.Marshal(policy)

	hash1, err1 := ComputePolicyHash(data1)
	hash2, err2 := ComputePolicyHash(data2)

	if err1 != nil || err2 != nil {
		t.Fatalf("failed to compute hash: %v, %v", err1, err2)
	}

	if hash1 != hash2 {
		t.Errorf("hashes don't match: %s != %s", hash1, hash2)
	}
}

func TestComputePolicyHash_DifferentPolicies(t *testing.T) {
	policy1 := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
			},
		},
		Default: "deny",
	}

	policy2 := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"user"},
				MethodsAny: []string{"GET"},
			},
		},
		Default: "deny",
	}

	data1, _ := json.Marshal(policy1)
	data2, _ := json.Marshal(policy2)

	hash1, _ := ComputePolicyHash(data1)
	hash2, _ := ComputePolicyHash(data2)

	if hash1 == hash2 {
		t.Error("hashes should differ for different policies")
	}
}

func TestComputePolicyHash_InvalidJSON(t *testing.T) {
	_, err := ComputePolicyHash([]byte(`{invalid}`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidatePolicyWithSize(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
			},
		},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs, hash, size := ValidatePolicyWithSize(data)

	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got %v", errs)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), size)
	}
}

func TestValidatePolicyWithSize_Invalid(t *testing.T) {
	policy := Policy{
		Version: 1,
		Mode:    "invalid",
		Default: "deny",
	}

	data, _ := json.Marshal(policy)
	errs, hash, size := ValidatePolicyWithSize(data)

	if len(errs) == 0 {
		t.Error("expected validation errors")
	}

	if hash != "" {
		t.Error("expected empty hash for invalid policy")
	}

	if size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), size)
	}
}

// Benchmark tests
func BenchmarkValidatePolicy(b *testing.B) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin", "user"},
				MethodsAny: []string{"GET", "POST", "PUT"},
				PathPrefix: "/api/v1",
			},
		},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidatePolicy(data)
	}
}

func BenchmarkComputePolicyHash(b *testing.B) {
	policy := Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []Rule{
			{
				Effect:     "allow",
				RolesAny:   []string{"admin"},
				MethodsAny: []string{"GET"},
				PathPrefix: "/api",
			},
		},
		Default: "deny",
	}

	data, _ := json.Marshal(policy)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputePolicyHash(data)
	}
}
