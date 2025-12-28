# Policy Validation — UMBRA-0001

## Overview

This document describes the server-side policy validation system implemented in UMBRA-0001. The validation ensures that policies are semantically valid for evaluation by the program, preventing crashes or undefined behavior during policy evaluation.

## Validation Rules

### Required Fields

All policies must include:
- `version` (integer > 0): Policy version, currently must be `1`
- `mode` (string): Policy language/mode, currently must be `"abac_v0"`
- `default` (string): Default decision when no rules match, must be `"allow"` or `"deny"`
- `rules` (array): Array of policy rules (must be an array, can be empty)

### Field Constraints

#### Policy Size
- **Maximum policy JSON size**: 10 MB (10,485,760 bytes)
- Enforced on write to prevent DoS and storage bloat

#### Default Field
- Must be one of: `"allow"` or `"deny"`
- Case-sensitive

#### Mode Field
- Must be exactly `"abac_v0"` (Attribute-Based Access Control, version 0)
- Validated to ensure policy is compatible with the evaluator

#### Rules Array
- **Maximum rule count**: 10,000 rules per policy
- Each rule must include at least one condition (see Rule Validation below)

### Rule Validation

Each rule in the `rules` array is validated:

#### Effect Field (Required)
- Must be `"allow"` or `"deny"`
- Case-sensitive

#### RolesAny Field (Optional)
- Array of role identifiers
- Each role must be a non-empty string
- No length limit per role, but must respect MaxStringLength (8,192 characters)

#### MethodsAny Field (Optional)
- Array of HTTP methods
- Valid values: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS` (case-insensitive)
- Each method must be a valid HTTP method
- Empty strings are rejected

#### PathPrefix Field (Optional)
- String to match against request paths
- Maximum length: 8,192 characters
- Can be empty string (matches all paths)

#### Condition Requirements
- Rules must specify at least one of: `roles_any`, `methods_any`, or `path_prefix`
- Rules with none of these fields are invalid

### String Length Validation

- **Maximum string field length**: 8,192 characters
- Applied to: role names, method names, path prefixes

## Canonical Hashing

### Hash Algorithm
- **Algorithm**: SHA-256
- **Output Format**: Hexadecimal (64-character string)

### Canonical Representation
1. Policy is unmarshaled from JSON
2. Re-marshaled with consistent formatting
3. All object keys are sorted
4. No unnecessary whitespace
5. Unicode escaping is disabled for readability

### Hash Properties
- **Deterministic**: Same policy always produces same hash
- **Immutable**: Policy hash never changes for a stored policy
- **Collision-resistant**: Different policies produce different hashes (in practice)

### Usage
- Stored in `policies.policy_hash` column
- Used in receipt hashing for audit trails
- Enables policy version verification without comparing full JSON

## Error Format

### Success (2xx)
Policy creation returns the created policy with:
```json
{
  "id": "uuid",
  "name": "policy-name",
  "version": 1,
  "active": false,
  "policy_hash": "sha256-hash",
  "policy": { ... },
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

### Validation Failure (400)
Invalid policies return HTTP 400 with:
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "request_id": "optional-request-id-from-header",
  "errors": [
    {
      "path": "version",
      "message": "version is required and must be > 0"
    },
    {
      "path": "rules[0].effect",
      "message": "effect must be 'allow' or 'deny', got 'maybe'"
    }
  ]
}
```

#### Error Code
- `POLICY_INVALID`: Policy validation failed

#### Error Path Format
- Root fields: `"fieldname"`
- Array elements: `"rules[0]"`
- Nested fields: `"rules[0].effect"`

### Other Errors
- `400 Bad Request`: Missing/invalid tenant, malformed JSON request body
- `503 Service Unavailable`: Database not configured
- `500 Internal Server Error`: Database errors during creation

## Endpoints

### Create Policy
**POST** `/v1/policies`

Validates and creates a new policy.

**Request:**
```json
{
  "name": "production-policy",
  "policy": { ... }
}
```

**Response (201 Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "production-policy",
  "version": 1,
  "active": false,
  "policy_hash": "abc123...",
  "policy": { ... },
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

**Response (400 Bad Request):**
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "errors": [ ... ]
}
```

### Simulate Policy
**POST** `/v1/policies/simulate`

Evaluates a policy against a decision request (supports both supplied and active policies).

**Request:**
```json
{
  "actor_roles": ["admin"],
  "method": "GET",
  "path": "/api/users",
  "policy": { ... }  // Optional; if omitted, uses active policy
}
```

**Response (200 OK):**
```json
{
  "decision": "allow",
  "reason": "allow rule matched",
  "rule_index": 0,
  "policy_hash": "abc123...",
  "policy_version": 1
}
```

**Response (400 Bad Request):**
If supplied `policy` fails validation:
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "errors": [ ... ]
}
```

## Safety Guarantees

### Invalid Policies Not Persisted
- Validation happens **before** database writes
- If validation fails, policy is **never** inserted into the database
- Atomic operation: validation + insert

### Policy Hash Integrity
- Hash is computed from canonical representation
- Stored hash matches what PDP receives
- Enables tamper detection (future feature)

### Backward Compatibility
- Existing policies in database are assumed valid
- New policies undergo strict validation
- Future validation endpoint for existing policies (not in V0)

## Known Limitations

### Not Validated
- Policy content is not evaluated for logical correctness
- Rules are not checked for conflicts or unreachability
- Path prefixes are not validated as valid URL patterns
- Role names have no validation against role registry (future)

### Future Enhancements
- Validate-on-read endpoint to check existing policies
- Maintenance command to validate entire policy dataset
- Policy versioning for backward compatibility
- Unknown field handling (currently rejected)

## Examples

### Valid Policy
```json
{
  "version": 1,
  "mode": "abac_v0",
  "rules": [
    {
      "effect": "deny",
      "roles_any": ["admin"],
      "methods_any": ["DELETE"],
      "path_prefix": "/api/v1/critical"
    },
    {
      "effect": "allow",
      "roles_any": ["user", "admin"],
      "methods_any": ["GET"],
      "path_prefix": "/api/v1/data"
    }
  ],
  "default": "deny"
}
```

### Invalid Policy (Missing Version)
```json
{
  "mode": "abac_v0",
  "rules": [],
  "default": "deny"
}
```

**Error Response:**
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "errors": [
    {
      "path": "version",
      "message": "version is required and must be > 0"
    }
  ]
}
```

### Invalid Policy (Invalid Effect)
```json
{
  "version": 1,
  "mode": "abac_v0",
  "rules": [
    {
      "effect": "maybe",
      "roles_any": ["admin"]
    }
  ],
  "default": "deny"
}
```

**Error Response:**
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "errors": [
    {
      "path": "rules[0].effect",
      "message": "effect must be 'allow' or 'deny', got 'maybe'"
    }
  ]
}
```

## Testing

### Unit Tests
Run validation tests:
```bash
go test ./packages/go/policy/... -v
```

Tests cover:
- Valid policies (no errors)
- Empty policies
- Invalid JSON
- Missing required fields
- Invalid enum values
- Size limits
- String length limits
- Deterministic hashing

### Integration Tests
Run HTTP API tests:
```bash
go test ./services/controlplane-api/internal/httpapi/... -v
```

Tests cover:
- Policy creation with validation
- Error response format
- Request without tenant header
- Simulate endpoint with supplied policy
- Simulate endpoint with invalid policy

### Manual Testing

#### Create Valid Policy
```bash
curl -X POST http://localhost:8080/v1/policies \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "name": "test-policy",
    "policy": {
      "version": 1,
      "mode": "abac_v0",
      "rules": [
        {
          "effect": "allow",
          "roles_any": ["admin"],
          "methods_any": ["GET"]
        }
      ],
      "default": "deny"
    }
  }'
```

Expected: `201 Created` with policy object including `policy_hash`

#### Create Invalid Policy
```bash
curl -X POST http://localhost:8080/v1/policies \
  -H "Content-Type: application/json" \
  -H "x-umbra-tenant-id: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "name": "invalid-policy",
    "policy": {
      "version": 1,
      "mode": "invalid_mode",
      "rules": [],
      "default": "deny"
    }
  }'
```

Expected: `400 Bad Request` with:
```json
{
  "code": "POLICY_INVALID",
  "message": "policy validation failed",
  "errors": [
    {
      "path": "mode",
      "message": "mode must be 'abac_v0', got 'invalid_mode'"
    }
  ]
}
```

## Maintenance

### Existing Policies
- Policies created before this feature are assumed valid
- No automatic validation of existing policies (V0)
- Manual audit recommended for pre-existing policies

### Migration Strategy
1. Deploy validation code
2. All new policies undergo validation
3. Optionally: Run maintenance validation on existing policies (future feature)
4. Document any policies found to be invalid

## References

- Policy types: `packages/go/policy/abac_v0.go`
- Validator implementation: `packages/go/policy/validator.go`
- API implementation: `services/controlplane-api/internal/httpapi/v0.go`
- OpenAPI spec: `docs/api/openapi.yaml`
