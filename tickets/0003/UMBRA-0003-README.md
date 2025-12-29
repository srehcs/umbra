# UMBRA-0003: Observe vs Enforce Mode (PEP_MODE)

## Ticket Summary
Implement configurable PEP_MODE to control enforcement behavior on policy DENY decisions:
- **observe**: Forward denied requests (audit/observability layer)
- **enforce**: Block denied requests (guardrail enforcement)

This enables Umbra to start as a monitoring tool and transition to active enforcement.

## Status: IMPLEMENTATION COMPLETE ✅

## Key Changes

### 1. PEP_MODE Configuration
- Read from environment variable: `PEP_MODE` (default: `observe`)
- Validated on startup with warning for invalid values
- Set in docker-compose for easy testing

### 2. Deny Decision Behavior
| Mode | Behavior | Status | Receipt Outcome |
|------|----------|--------|-----------------|
| observe | Forward request | upstream status | "forwarded" |
| enforce | Block request | 403 Forbidden | "blocked" |

### 3. Allow Decision Behavior
Both modes: Forward request (no difference)

### 4. Response Contract (Enforce + Deny)
```json
{
  "error_code": "POLICY_DENIED",
  "message": "policy reason",
  "request_id": "correlation-id",
  "decision_id": "decision-uuid"
}
```

### 5. Receipt Fields Added
```json
{
  "pep.mode": "observe|enforce",
  "enforcement.outcome": "blocked|forwarded"
}
```

## Files Modified
- `services/pep-gateway/internal/httpapi/v0.go` - Core implementation
- `deployments/docker-compose.yml` - Default configuration
- `services/pep-gateway/internal/httpapi/v0_test.go` - New tests

## How to Test

### Quick Verify (after `make dev` + `make seed`)

**Test 1: Observe Mode**
```bash
# Create a deny policy, then:
curl -X POST http://localhost:8082/demo \
  -H "x-umbra-tenant-id: $(uuidgen)" \
  -H "content-type: application/json" \
  -d '{"tool":"test","method":"GET","path":"/x","actor":{"id":"u1","roles":["dev"]}}'
# Expected: 200 OK (denied but forwarded)
```

**Test 2: Enforce Mode**
```bash
# Change PEP_MODE=enforce in docker-compose, restart pep-gateway
# Same request as above
# Expected: 403 Forbidden with POLICY_DENIED error
```

### Verification Script
See `verify-0003.sh` for automated verification

### Integration Tests
```bash
cd /workspaces/umbra/identity/control-plane/services/pep-gateway
go test -v ./internal/httpapi -run TestObserveVsEnforceMode
```

## Risks & Mitigation
- **Observe mode allows denied requests**: Design feature, controlled by PEP_MODE flag
- **Default observe mode**: Safe for initial deployment
- **Mode switch requires restart**: Expected for env var changes

## Rollback
Change `PEP_MODE` env var and restart `pep-gateway` service

---
See `IMPLEMENTATION-0003.md` for full details
