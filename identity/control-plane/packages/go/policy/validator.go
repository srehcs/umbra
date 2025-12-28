package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// PolicyMaxBytes is the maximum size of a policy document in bytes (10 MB)
	PolicyMaxBytes = 10 * 1024 * 1024

	// MaxRules is the maximum number of rules in a policy
	MaxRules = 10000

	// MaxStringLength is the maximum length of string fields
	MaxStringLength = 8192

	// ErrorCodePolicyInvalid is the error code for policy validation failures
	ErrorCodePolicyInvalid = "POLICY_INVALID"
)

// ValidationError represents a single field-level validation error
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// PolicyValidationResponse is the response format for validation failures
type PolicyValidationResponse struct {
	Code       string             `json:"code"`
	Message    string             `json:"message"`
	RequestID  string             `json:"request_id,omitempty"`
	Errors     []ValidationError  `json:"errors"`
}

// ValidatePolicy validates a policy document and returns any errors
func ValidatePolicy(data []byte) []ValidationError {
	var errs []ValidationError

	// Check size first
	if len(data) == 0 {
		errs = append(errs, ValidationError{
			Path:    "policy",
			Message: "policy cannot be empty",
		})
		return errs
	}

	if len(data) > PolicyMaxBytes {
		errs = append(errs, ValidationError{
			Path:    "policy",
			Message: fmt.Sprintf("policy size %d exceeds maximum of %d bytes", len(data), PolicyMaxBytes),
		})
		return errs
	}

	// Unmarshal and validate structure
	var pol Policy
	if err := json.Unmarshal(data, &pol); err != nil {
		errs = append(errs, ValidationError{
			Path:    "policy",
			Message: fmt.Sprintf("invalid JSON: %v", err),
		})
		return errs
	}

	// Validate required fields
	if pol.Version == 0 {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: "version is required and must be > 0",
		})
	}

	if pol.Mode == "" {
		errs = append(errs, ValidationError{
			Path:    "mode",
			Message: "mode is required",
		})
	} else if pol.Mode != "abac_v0" {
		errs = append(errs, ValidationError{
			Path:    "mode",
			Message: fmt.Sprintf("mode must be 'abac_v0', got '%s'", pol.Mode),
		})
	}

	// Validate default value
	if pol.Default == "" {
		errs = append(errs, ValidationError{
			Path:    "default",
			Message: "default is required",
		})
	} else if pol.Default != "allow" && pol.Default != "deny" {
		errs = append(errs, ValidationError{
			Path:    "default",
			Message: fmt.Sprintf("default must be 'allow' or 'deny', got '%s'", pol.Default),
		})
	}

	// Validate rules
	if pol.Rules == nil {
		errs = append(errs, ValidationError{
			Path:    "rules",
			Message: "rules is required and must be an array",
		})
	} else if len(pol.Rules) > MaxRules {
		errs = append(errs, ValidationError{
			Path:    "rules",
			Message: fmt.Sprintf("rules array exceeds maximum length of %d", MaxRules),
		})
	} else {
		ruleErrs := validateRules(pol.Rules)
		errs = append(errs, ruleErrs...)
	}

	return errs
}

// validateRules validates all rules in the policy
func validateRules(rules []Rule) []ValidationError {
	var errs []ValidationError

	for i, rule := range rules {
		prefix := fmt.Sprintf("rules[%d]", i)

		// Validate effect
		if rule.Effect == "" {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("%s.effect", prefix),
				Message: "effect is required",
			})
		} else if rule.Effect != "allow" && rule.Effect != "deny" {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("%s.effect", prefix),
				Message: fmt.Sprintf("effect must be 'allow' or 'deny', got '%s'", rule.Effect),
			})
		}

		// Validate string length constraints
		if len(rule.PathPrefix) > MaxStringLength {
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("%s.path_prefix", prefix),
				Message: fmt.Sprintf("path_prefix length %d exceeds maximum of %d", len(rule.PathPrefix), MaxStringLength),
			})
		}

		// Validate roles_any
		if rule.RolesAny != nil {
			for j, role := range rule.RolesAny {
				if role == "" {
					errs = append(errs, ValidationError{
						Path:    fmt.Sprintf("%s.roles_any[%d]", prefix, j),
						Message: "role cannot be empty string",
					})
				}
				if len(role) > MaxStringLength {
					errs = append(errs, ValidationError{
						Path:    fmt.Sprintf("%s.roles_any[%d]", prefix, j),
						Message: fmt.Sprintf("role length %d exceeds maximum of %d", len(role), MaxStringLength),
					})
				}
			}
		}

		// Validate methods_any
		if rule.MethodsAny != nil {
			for j, method := range rule.MethodsAny {
				if method == "" {
					errs = append(errs, ValidationError{
						Path:    fmt.Sprintf("%s.methods_any[%d]", prefix, j),
						Message: "method cannot be empty string",
					})
				}
				if len(method) > MaxStringLength {
					errs = append(errs, ValidationError{
						Path:    fmt.Sprintf("%s.methods_any[%d]", prefix, j),
						Message: fmt.Sprintf("method length %d exceeds maximum of %d", len(method), MaxStringLength),
					})
				}
				// Validate HTTP method enum
				validMethod := false
				upperMethod := strings.ToUpper(method)
				for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
					if upperMethod == m {
						validMethod = true
						break
					}
				}
				if !validMethod {
					errs = append(errs, ValidationError{
						Path:    fmt.Sprintf("%s.methods_any[%d]", prefix, j),
						Message: fmt.Sprintf("method '%s' is not a valid HTTP method", method),
					})
				}
			}
		}

		// At least one condition should be specified
		if len(rule.RolesAny) == 0 && len(rule.MethodsAny) == 0 && rule.PathPrefix == "" {
			errs = append(errs, ValidationError{
				Path:    prefix,
				Message: "rule must specify at least one of: roles_any, methods_any, or path_prefix",
			})
		}
	}

	return errs
}

// ComputePolicyHash computes a canonical SHA256 hash of a policy
func ComputePolicyHash(data []byte) (string, error) {
	// Unmarshal and re-marshal to ensure canonical form (sorted keys, compact output)
	var pol Policy
	if err := json.Unmarshal(data, &pol); err != nil {
		return "", fmt.Errorf("failed to parse policy: %w", err)
	}

	// Re-marshal in canonical form with sorted keys and no whitespace
	canonical, err := json.Marshal(pol)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalize policy: %w", err)
	}

	// Ensure deterministic output by re-normalizing with canonical JSON encoder
	canonical, err = canonicalizeJSON(canonical)
	if err != nil {
		return "", fmt.Errorf("failed to normalize JSON: %w", err)
	}

	h := sha256.Sum256(canonical)
	return hex.EncodeToString(h[:]), nil
}

// canonicalizeJSON normalizes JSON to a deterministic form with sorted object keys
func canonicalizeJSON(data []byte) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}

	// Re-marshal with sorted keys in a compact form
	// json.Marshal already produces sorted keys for maps
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}

	// Remove trailing newline added by Encoder
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// ValidatePolicyWithSize validates a policy and returns both validation errors and the canonical hash
func ValidatePolicyWithSize(data []byte) ([]ValidationError, string, int64) {
	errs := ValidatePolicy(data)
	hash := ""
	if len(errs) == 0 {
		h, _ := ComputePolicyHash(data)
		hash = h
	}
	return errs, hash, int64(len(data))
}
