# Backlog (Epics → Stories)

This backlog is designed to produce a **demo-worthy vertical slice** while enforcing `RULES.md` (security/reliability/observability as product features).

## Story template (required fields)
- **Why:** business/demo value
- **Scope:** what changes
- **Acceptance Criteria:** measurable
- **Security note:** threat/abuse cases touched
- **Telemetry note:** traces/log attrs/metrics added
- **Rollback:** what to do if it breaks

---

## Demo definition (V0-C “good demo spot”)
**Goal:** prove the choke point end-to-end.

1. Admin registers a Tool + Policy in Control Plane
2. Agent/runtime sends a tool invocation → **PEP**
3. PEP calls **PDP** `/v1/decision` and enforces allow/deny
4. PDP writes **decision receipt** + PEP writes **invocation receipt** (hash-chained)
5. UI shows last N receipts + detail view
6. Jaeger shows trace across PEP → PDP → upstream
7. Tenant isolation demonstrated (2 tenants; no cross-tenant reads, tested)

---

## EPIC 0 — Process + docs hygiene (Rules-first)
### E0-S1 Add process docs + DoD
**Files**
- `docs/process/*`, `docs/README.md`

**Acceptance Criteria**
- DoD aligns with RULES.md checklists
- Backlog defines story template & required fields

**Security note**
- None (process), but ensures future work is auditable.

**Telemetry note**
- None.

### E0-S2 Bring “Umbra umbrella docs” into repo (context stays current)
**Files**
- `umbra/README.md` (index)
- `docs/README.md` links

**Acceptance Criteria**
- Umbra docs are discoverable and linked
- Clear “source of truth” statement

---

## EPIC 1 — Contracts + architecture lock-in
### E1-S1 Harden OpenAPI (contract-first)
**Files**
- `docs/api/openapi.yaml`

**Acceptance Criteria**
- `/v1/decision`, `/v1/policies`, `/v1/tools` fully specified incl. error model
- Required headers/correlation documented (`traceparent`, `x-umbra-request-id`, tenant mechanism)

**Security note**
- Prevents auth/tenant drift by codifying required context.

**Telemetry note**
- Correlation IDs required in contract.

### E1-S2 Actionable threat model & trust boundaries
**Files**
- `docs/architecture/threat-model.md`
- `docs/architecture/trust-boundaries.md`

**Acceptance Criteria**
- “Fail closed” decision documented
- “Never log” list explicit
- Trust boundaries match actual call graph

### E1-S3 ADR minimum set (no rework)
**Files**
- `docs/adr/0005-policy-evaluator-interface.md` (new)
- `docs/adr/0006-redis-streams-events.md` (new)
- `docs/adr/0007-service-to-service-auth-plan.md` (new, stub ok)

**Acceptance Criteria**
- Alternatives + consequences included
- Swappable evaluator interface defined (ABAC now → OPA later)
- Redis Streams event types defined

---

## EPIC 2 — Data model + migrations + seed
### E2-S1 Add tenant-aware schema + migrations
**Files**
- `services/**/migrations/*`
- `Makefile` add migrate targets

**Acceptance Criteria**
- Tables: tenants/tools/policies/receipts_decision/receipts_invocation
- Every table has `tenant_id` (where applicable) + indexes
- Receipts have `prev_hash`, `hash`, signing-ready fields

**Security note**
- Tenant isolation is a top risk; schema enforces it.

### E2-S2 Deterministic seed (2 tenants, sample tool/policy)
**Files**
- `scripts/dev/seed_db.sh`

**Acceptance Criteria**
- `make seed` creates TenantA + TenantB with distinct data
- Outputs IDs for demo

### E2-S3 Add deterministic upstream service for demo
**Files**
- `deployments/docker-compose.yml`

**Acceptance Criteria**
- Demo does not depend on external services
- PEP can forward to upstream container in compose

---

## EPIC 3 — Vertical slice behavior (PEP↔PDP↔DB + receipts + traces)
### E3-S1 PDP evaluates ABAC and writes decision receipts
**Files**
- `services/pdp/internal/decision/*` (new)
- `services/pdp/internal/storage/*` (new)
- `services/pdp/internal/httpapi/v0.go`

**Acceptance Criteria**
- Loads active policy for tenant
- Default-deny
- Decision receipt persisted + hash-chained
- Unit tests for evaluator + receipt hashing determinism

**Telemetry note**
- Span attributes: tenant_id, decision_id, policy_hash

### E3-S2 PEP enforces and writes invocation receipts
**Files**
- `services/pep-gateway/internal/client/*` (new)
- `services/pep-gateway/internal/receipts/*` (new)
- `services/pep-gateway/internal/httpapi/v0.go`

**Acceptance Criteria**
- Calls PDP w/ timeout + bounded retry
- Fail-closed default when PDP unreachable
- Forwards when allowed
- Invocation receipt persisted + hash-chained

### E3-S3 Control Plane CRUD (tools + policies)
**Files**
- `services/controlplane-api/internal/storage/*` (new)
- `services/controlplane-api/internal/httpapi/v0.go`

**Acceptance Criteria**
- CRUD for tools/policies is real (tenant-scoped)
- Integration tests prove tenant A cannot see tenant B

---

## EPIC 4 — UI (minimal but enterprise-cute)
### E4-S1 UI pages: receipts feed + detail; tools/policies list/create
**Files**
- `ui/app/*`

**Acceptance Criteria**
- Receipts feed shows last N
- Receipt detail shows policy_hash, decision_id, obligations, trace_id
- Basic forms for tools/policies

### E4-S2 OpenAPI → TS contract generation (no drift)
**Files**
- `packages/contracts/*` (generated)
- CI checks generated output

**Acceptance Criteria**
- `make gen` produces deterministic output
- CI fails on drift

---

## EPIC 5 — Demo hardening
### E5-S1 Demo runbook
**Files**
- `docs/runbooks/demo.md` (new)

**Acceptance Criteria**
- Step-by-step script for the demo + expected outputs
- Includes Jaeger trace walkthrough

---

## Suggested PR slicing (shippable increments)
1) Epic 0 (process docs + docs index + umbrella index)
2) E1-S1 OpenAPI harden
3) E2 schema + migrations + migrate targets
4) Seed + deterministic upstream
5) PDP evaluator + decision receipts
6) PEP enforcement + invocation receipts
7) Control plane CRUD + tenant tests
8) UI receipts feed + detail
9) Contract generation + CI drift gate
10) Demo runbook + CI hardening
