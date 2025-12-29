# UMBRA-0001 Quick Reference

## What Was Implemented

### ✅ Server-Side Policy Validation
Prevents invalid policies from being persisted or causing evaluation crashes.

### Files Created
1. `packages/go/policy/validator.go` - Validation + hashing logic
2. `packages/go/policy/validator_test.go` - 23 comprehensive unit tests
3. `services/controlplane-api/internal/httpapi/v0_test.go` - Integration tests
4. `docs/policy-validation.md` - Complete validation reference

### Files Modified
1. `services/controlplane-api/internal/httpapi/v0.go` - Added validation to create/simulate endpoints

## Validation Rules Summary

### Required Fields
- `version` (must be 1)
- `mode` (must be "abac_v0")
- `default` (must be "allow" or "deny")
- `rules` (array of rules)

### Limits
- Policy size: ≤ 10 MB
- Rules per policy: ≤ 10,000
- String length: ≤ 8,192 characters

### Rule Validation
- `effect` required: "allow" or "deny"
- `methods_any` optional: Valid HTTP methods (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)
- `roles_any` optional: Array of non-empty strings
- `path_prefix` optional: String
- **At least one condition required** per rule

## Error Response Format

```json
HTTP 400 Bad Request
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "request_id": "optional",
  "errors": [
    {"path": "rules[0].effect", "message": "effect must be 'allow' or 'deny', got 'maybe'"}
  ]
}
```

## Hash Algorithm
- **SHA-256** (deterministic)
- **Canonical format** (sorted keys, consistent serialization)
- **Hex output** (64-character string)

## Endpoints Modified

### POST /v1/policies
- Now validates policy before persisting
- Computes canonical hash
- Returns 400 with field-level errors if invalid
- Returns 201 with policy object if valid

### POST /v1/policies/simulate
- Validates supplied policies before evaluation
- Returns 400 with POLICY_INVALID if supplied policy fails validation
- Uses active policy if none supplied
- Returns decision with policy hash and version

## Testing

### Run Unit Tests
```bash
cd identity/control-plane
go test -v ./packages/go/policy/...
go test -v ./services/controlplane-api/internal/httpapi/...
```

### Run Integration Tests (requires running service)
```bash
bash integration-test-0001.sh
```

### Manual Test: Create Invalid Policy
```bash
curl -X POST http://localhost:8080/v1/policies \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "name": "test",
    "policy": {"version": 1, "mode": "invalid", "default": "deny"}
  }'
# Expected: 400 with POLICY_INVALID
```

### Manual Test: Create Valid Policy
```bash
curl -X POST http://localhost:8080/v1/policies \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "name": "test",
    "policy": {
      "version": 1,
      "mode": "abac_v0",
      "rules": [{"effect": "allow", "roles_any": ["admin"], "methods_any": ["GET"]}],
      "default": "deny"
    }
  }'
# Expected: 201 with policy_hash
```

## Documentation

Complete validation documentation available at:
- **docs/policy-validation.md** - Full reference guide with examples
- **IMPLEMENTATION-0001.md** - Implementation details and status

## Verification Checklist

- ✅ Malformed policy JSON → 400 with POLICY_INVALID + field errors
- ✅ Valid policy → 201 with stored hash
- ✅ Policy evaluation never crashes due to shape errors
- ✅ Hashes are canonical and deterministic
- ✅ Invalid policies never persisted
- ✅ Structured error responses with field paths
- ✅ All tests passing
- ✅ Comprehensive documentation

## Non-Features (Intentional)

- Policy logical correctness validation
- Rule conflict detection
- Future: validate-on-read for existing policies
- Future: bulk migration/validation tools
