# UMBRA-0003 Implementation Summary: Observe vs Enforce Mode (PEP_MODE)

## Overview
Implemented PEP_MODE configuration to differentiate Umbra's behavior when policy enforcement decisions are DENY:
- **observe** mode: forwards denied requests (watching/audit layer)
- **enforce** mode: blocks denied requests (guardrail enforcement)

This allows users to deploy Umbra initially as observability layer, then transition to guardrail enforcement.

## Delivery Criteria Status

### ✅ Config - PEP_MODE Environment Variable
- **Location**: `/workspaces/umbra/identity/control-plane/services/pep-gateway/cmd/server/main.go` (via v0.go)
- **Values**: `observe` (default) or `enforce`
- **Default**: `observe` (safe default for observation)
- **Validation**: Invalid values log warning and default to observe
- **Docker Configuration**: Set in `docker-compose.yml` environment

### ✅ Behavioral Guarantees - DENY Decision

#### Observe Mode
- **Behavior**: Forwards request to upstream
- **Receipt**: `enforcement.outcome: "forwarded"`, `pep.mode: "observe"`, `decision: "deny"`
- **Status Code**: Same as upstream (e.g., 200, 500, etc.)
- **Use Case**: Monitor tool usage without blocking

#### Enforce Mode
- **Behavior**: Blocks request with structured error response
- **Receipt**: `enforcement.outcome: "blocked"`, `pep.mode: "enforce"`, `decision: "deny"`
- **Status Code**: `403 Forbidden`
- **Response Format**: See response contract below

### ✅ Behavioral Guarantees - ALLOW Decision
- **Both Modes**: Forward request to upstream
- **Receipt**: `enforcement.outcome: "forwarded"`, `pep.mode: (observe|enforce)`, `decision: "allow"`
- **Status Code**: Same as upstream
- **Consistency**: No difference between modes for allowed requests

### ✅ Response Contract for Blocked Requests (enforce mode)
```json
{
  "error_code": "POLICY_DENIED",
  "message": "reason from policy decision",
  "request_id": "x-umbra-request-id or generated UUID",
  "decision_id": "decision ID from PDP (optional if not generated)"
}
```
- **Error Code**: Stable `POLICY_DENIED` code
- **Request ID**: Always included for correlation
- **Decision ID**: Included for audit trail
- **Sensitive Args**: Tool arguments NOT echoed in response

### ✅ Receipts Include Mode and Outcome
Receipt body now includes:
```json
{
  "tool": "tool-name",
  "method": "GET",
  "path": "/api/resource",
  "outcome": "denied|success|error",
  "pep.mode": "observe|enforce",
  "enforcement.outcome": "blocked|forwarded",
  "status_code": 200,
  "latency_ms": 42,
  "started_at": "2025-12-29T14:30:00Z",
  "meta": {...}
}
```

Database remains backward compatible (new fields are optional JSON).

## Files Changed

### Core Implementation
1. **pep-gateway/internal/httpapi/v0.go**
   - Added `invocationReceiptBody.PEPMode` and `Enforcement` fields
   - Added `blockedResponse` struct with POLICY_DENIED response contract
   - Updated `registerV0()` to read and validate `PEP_MODE` env var
   - Updated `handleToolProxy()` to accept `pepMode` parameter
   - Implemented observe/enforce logic in DENY handling:
     - **observe**: forward and record `enforcement=forwarded`
     - **enforce**: block and record `enforcement=blocked` with structured error
   - Updated `writeInvocationReceipt()` to accept and store `pepMode` and `enforcement`

### Configuration
2. **deployments/docker-compose.yml**
   - Added `PEP_MODE: observe` environment variable to `pep-gateway` service
   - Default to observe mode for safe out-of-box behavior

### Tests
3. **pep-gateway/internal/httpapi/v0_test.go** (new file)
   - Test suite covering:
     - Observe mode with DENY: forwards with 200 status
     - Enforce mode with DENY: blocks with 403 status
     - Both modes with ALLOW: forward with 200 status
     - Response format validation for blocked requests

## Architecture Notes

### Design Decisions
1. **PEP_MODE as environment variable**: Simple, well-understood deployment pattern
2. **Default to observe**: Safe default prevents surprise blocking in production
3. **Structured DENY handling**: Separate code paths for observe vs enforce make logic clear
4. **Receipt fields for both modes**: Complete audit trail regardless of mode
5. **No request modifications**: In observe mode, request forwarded unchanged (no special headers)

### Correlation
- Request ID (`x-umbra-request-id`) always included in:
  - Receipt body
  - Error response (when blocked)
  - Trace context (OpenTelemetry)
- Decision ID included in error response for audit trail

## How to Verify

### Prerequisites
```bash
cd /workspaces/umbra/identity/control-plane
make dev
make seed
```

### 1. Observe Mode (Forward Denied)
```bash
export PEP_MODE=observe
make dev

# In another terminal, create a deny policy, then:
curl -X POST http://localhost:8082/demo \
  -H "x-umbra-tenant-id: <tenant-id>" \
  -H "content-type: application/json" \
  -d '{
    "tool": "restricted-tool",
    "method": "GET",
    "path": "/sensitive",
    "actor": {"id": "user-1", "roles": ["developer"]}
  }'

# Expected: 200 OK (request forwarded despite policy denial)
# Check receipt in database: enforcement.outcome should be "forwarded"
```

### 2. Enforce Mode (Block Denied)
```bash
export PEP_MODE=enforce
make dev

# Same request as above:
curl -X POST http://localhost:8082/demo \
  -H "x-umbra-tenant-id: <tenant-id>" \
  -H "content-type: application/json" \
  -d '{
    "tool": "restricted-tool",
    "method": "GET",
    "path": "/sensitive",
    "actor": {"id": "user-1", "roles": ["developer"]}
  }'

# Expected: 403 Forbidden with:
# {
#   "error_code": "POLICY_DENIED",
#   "message": "policy reason",
#   "request_id": "...",
#   "decision_id": "..."
# }
# Check receipt in database: enforcement.outcome should be "blocked"
```

### 3. Allow in Both Modes (Same Behavior)
```bash
# Create allow policy, test with same request in both modes
# Expected in both cases: 200 OK with upstream response
# Receipt in both cases: enforcement.outcome should be "forwarded"
```

### 4. Verify Receipt Fields
```sql
-- From control-plane database
SELECT pep_mode, enforcement_outcome, decision, tool_name
FROM receipts_invocation
WHERE tool_name = 'restricted-tool'
ORDER BY ts DESC LIMIT 10;

-- Verify:
-- All rows have pep.mode populated
-- DENY rows show enforcement.outcome as "blocked" or "forwarded" depending on mode
-- ALLOW rows show enforcement.outcome as "forwarded" in both modes
```

### 5. Trace Correlation
```bash
# Check Jaeger UI at http://localhost:16686
# Search for traces with tag pep.mode=observe or pep.mode=enforce
# Verify trace shows correct span attributes and decision flow
```

## Risks and Rollback

### Risk: Blocking Critical Tool Calls
**Scenario**: User deploys with `PEP_MODE=enforce` before policies are correctly configured
**Impact**: Tool calls fail unexpectedly
**Mitigation**: 
- Default is `observe` mode
- Recommend testing mode changes on non-prod first
- Receipts show enforcement outcome for immediate visibility

### Risk: Observability Gap in Observe Mode
**Scenario**: User relies on observe mode without setting up alert for denied requests
**Impact**: Policy violations go unnoticed
**Mitigation**:
- Receipt `enforcement.outcome="forwarded"` makes denial visible in logs/dashboards
- `decision="deny"` in receipt clearly indicates policy violation
- Recommend monitoring receipts with outcome=denied

### Rollback Strategy
1. **To switch modes**: Change `PEP_MODE` environment variable and restart `pep-gateway`
2. **To verify rollback**: Check new requests have correct `pep.mode` in receipts
3. **No data migration needed**: Receipt schema already supports both modes
4. **Quick flag**: Mode is purely behavioral, no state changes required

## Testing Checklist

- [ ] Unit tests in `v0_test.go` pass
- [ ] Integration tests verify observe vs enforce behavior
- [ ] Observe mode forwards denied requests
- [ ] Enforce mode blocks denied requests with POLICY_DENIED
- [ ] Allow decisions same in both modes
- [ ] Receipt includes `pep.mode` and `enforcement.outcome`
- [ ] Request ID flows through error response
- [ ] Decision ID flows through error response
- [ ] Sensitive tool args not echoed in response
- [ ] Trace spans include `pep.mode` attribute
- [ ] docker-compose sets `PEP_MODE=observe` default
- [ ] Invalid PEP_MODE values default to observe with warning

## Implementation Time
Estimated: 2-3 hours for full implementation + testing
