# Umbra Tickets

Source: GitHub issues (copied into this file for planning).
Open items are scoped as the next steps to make the product demo-ready.

Archive normalization note (2026-04-27):

- This file is planning history, not the canonical source of current product status.
- Historical references to direct `x-umbra-user` / `x-umbra-roles` / `x-umbra-tenant-id` trust should be read as superseded planning context.
- The current implementation uses verified bearer-token claims for auth-enabled UI/API flows, while `x-umbra-tenant-id` remains only a local dev/demo path when auth is disabled.

## Open tickets (demo readiness)

### Demo-critical

### UMBRA-0032: Operational Readiness Pack (One-Command Bring-up)

Status: Completed (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Reliable, repeatable environment setup reduces friction for evaluation and internal demos.

Scope

- Add a `make demo` (or `scripts/dev/demo_start.sh`) that runs `make dev`, `make seed`, waits for health, and runs `demo_check`.
- Print tenant IDs and a minimal set of curl commands for allow/deny + receipts lookup.
- Update `docs/runbooks/demo.md` to reference the one-command flow and expected outputs.

Likely files

- `identity/control-plane/Makefile`
- `identity/control-plane/scripts/dev/demo_check.sh`
- New: `identity/control-plane/scripts/dev/demo_start.sh`
- `identity/control-plane/docs/runbooks/demo.md`

Acceptance Criteria

- One command brings the stack up, seeds data, and prints the verification steps.
- `demo_check` passes and services are healthy.
- Steps match actual responses (allow/deny + receipts).

Tests

- Script smoke test (local).

Verification Steps

- Run the command and follow the printed steps end-to-end.

Implementation Notes (2026-03-03)

- Added `make demo` to `identity/control-plane/Makefile`.
- Added `identity/control-plane/scripts/dev/demo_start.sh` to run compose up, readiness wait, seed, and `demo-check`, then print tenant IDs and verification commands.
- Updated `identity/control-plane/docs/runbooks/demo.md` to make one-command flow canonical and clarify expected deny behavior in enforce vs observe mode.
- Updated `identity/control-plane/deployments/docker-compose.yml` so `PEP_MODE` is env-overridable (`\${PEP_MODE:-observe}`) for deterministic demo runs.
- Updated `identity/control-plane/docs/how-to-develop.md` to include `make demo`.

Local Verification Run

- `make demo`
- `bash -n identity/control-plane/scripts/dev/demo_start.sh`
- `docker compose -f identity/control-plane/deployments/docker-compose.yml config`
- `curl -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/demo` -> `200`, body `hello-from-upstream`
- `curl -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/secret` -> `403`, `error.code=POLICY_DENIED`
- `curl -X POST -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100"` -> `{"ok":true,...}`

GitHub Update Draft

```markdown
Ticket: UMBRA-0032 — Operational Readiness Pack (One-Command Bring-up)
Status: Done

Summary

- Added `make demo` to run one-command bring-up.
- Added `scripts/dev/demo_start.sh` to start, wait, seed, run demo checks, and print tenant IDs plus verification commands.
- Updated `docs/runbooks/demo.md` and `docs/how-to-develop.md` to reference the one-command flow.
- Made `PEP_MODE` env-overridable in compose for deterministic enforce-mode demos.

Verification

- `make demo` completed successfully end-to-end on local.
- Allow call: `GET /tool/demo` returned `200` with `hello-from-upstream`.
- Deny call: `GET /tool/secret` returned `403` with `error.code=POLICY_DENIED`.
- Receipts integrity verify returned `ok=true`.
```

---

### UMBRA-0033: Trace + Observability Verification Runbook

Status: Completed (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Trace continuity must be provable for incident response and evaluation scenarios.

Scope

- Add a short runbook that generates a trace and links it to the receipt `trace_id`.
- Provide a Jaeger query example and expected fields (trace_id, span_id, request_id).
- Optional: add a small script that prints the trace ID and a Jaeger link.

Likely files

- New: `identity/control-plane/docs/runbooks/trace_demo.md`
- `identity/control-plane/docs/runbooks/demo.md`
- Optional: `identity/control-plane/scripts/dev/trace_smoke.sh`

Acceptance Criteria

- Steps show where the trace appears in Jaeger and in receipts.
- Clear, copy-pasteable instructions with expected output fields.

Tests

- Manual runbook verification.

Verification Steps

- Execute the runbook and confirm trace/receipt correlation.

Implementation Notes (2026-03-03)

- Added `identity/control-plane/docs/runbooks/trace_demo.md` with:
  - script-driven correlation flow (`trace_smoke.sh`)
  - manual correlation flow (`request_id` -> receipts -> Jaeger)
  - expected output fields: `trace_id`, `span_id`, `request_id`, `decision_id`
- Added `identity/control-plane/scripts/dev/trace_smoke.sh` to:
  - send a PEP request with deterministic `x-umbra-request-id`
  - query `GET /v1/receipts?q=<request_id>`
  - print correlation fields and Jaeger lookup instructions
- Linked trace runbook from:
  - `identity/control-plane/docs/README.md`
  - `identity/control-plane/README.md`
  - `identity/control-plane/docs/runbooks/demo.md`

Local Verification Run

- `bash -n identity/control-plane/scripts/dev/trace_smoke.sh`
- `TENANT_ID=<TenantA> bash identity/control-plane/scripts/dev/trace_smoke.sh` -> `403` and non-empty `trace_id`, `span_id`, `decision_id`
- `TENANT_ID=<TenantA> TRACE_KIND=allow bash identity/control-plane/scripts/dev/trace_smoke.sh` -> `200` and non-empty `trace_id`, `span_id`, `decision_id`

GitHub Update Draft

```markdown
Ticket: UMBRA-0033 — Trace + Observability Verification Runbook
Status: Done

Summary

- Added `docs/runbooks/trace_demo.md` with copy-pasteable steps for trace/receipt correlation.
- Added `scripts/dev/trace_smoke.sh` to generate a request, read receipt correlation fields, and print Jaeger lookup guidance.
- Documented expected fields (`request_id`, `trace_id`, `span_id`, `decision_id`) and linked runbook from docs index/demo docs.

Verification

- Deny smoke run returned `403` and produced correlation fields in receipts.
- Allow smoke run returned `200` and produced correlation fields in receipts.
- Script output includes a concrete trace ID that can be located in Jaeger (`pep-gateway.http`).
```

---

### UMBRA-0034: Evaluation Walkthrough + Expected Output Pack

Status: Completed (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Stakeholders need a concise, confidence-building walkthrough with expected outputs and explanations.

Scope

- Create a concise walkthrough with 5–7 steps (policy → decision → enforce → receipt → verification).
- Include expected JSON snippets and UI screenshot placeholders (no secrets).
- Link to relevant APIs and receipts verification endpoints.

Likely files

- New: `identity/control-plane/docs/runbooks/demo_walkthrough.md`
- `identity/control-plane/docs/README.md`

Acceptance Criteria

- Walkthrough can be followed without prior context.
- Expected outputs align with seeded data and current API responses.

Tests

- Dry-run the walkthrough locally.

Verification Steps

- Follow the walkthrough and confirm outputs match.

Implementation Notes (2026-03-03)

- Added `identity/control-plane/docs/runbooks/demo_walkthrough.md` with a concise 6-step flow:
  policy/tool baseline -> allow -> deny -> receipts correlation -> receipts verify -> UI confirmation.
- Included expected JSON snippets and explicit API endpoint references used in the flow.
- Added UI screenshot placeholders for evaluator packets without storing sensitive artifacts.
- Linked walkthrough from:
  - `identity/control-plane/docs/README.md`
  - `identity/control-plane/README.md`
  - `identity/control-plane/docs/runbooks/demo.md`

Local Verification Run

- `make demo`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" http://localhost:8080/v1/tools | jq .`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" http://localhost:8080/v1/policies | jq .`
- `curl -i -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/demo` -> `200`
- `curl -i -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/secret` -> `403`, `error.code=POLICY_DENIED`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts?limit=3" | jq .`
- `curl -s -X POST -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100"` -> `{"ok":true,...}`

GitHub Update Draft

```markdown
Ticket: UMBRA-0034 — Evaluation Walkthrough + Expected Output Pack
Status: Done

Summary

- Added `docs/runbooks/demo_walkthrough.md` with a concise evaluator walkthrough:
  policy/tool baseline, allow/deny calls, receipts correlation, and integrity verification.
- Included expected output snippets and direct endpoint references for repeatability.
- Added screenshot placeholders for UI evidence packet capture.
- Linked the walkthrough from docs indexes and demo runbook.

Verification

- Executed walkthrough against local stack started with `make demo`.
- Baseline calls returned seeded tool/policy data.
- Allow call returned `200`; deny call returned `403` with `POLICY_DENIED`.
- Receipts list showed correlation fields (`request_id`, `decision_id`, `trace_id`).
- Receipts verify endpoint returned `ok=true`.
```

---

### UMBRA-0035: Demo Reset + Seed Profiles

Status: Open  
Author: TBD  
Opened: TBD

Why  
Reliable demos need a fast, repeatable way to reset state and reseed known-good data.

Scope

- Add a reset script that wipes demo data and re-seeds the default tenant/policies/tools.
- Provide seed profiles (default, deny-heavy, allow-heavy) for varied demos.
- Document when to use reset vs seed-only flows.

Likely files

- `identity/control-plane/scripts/dev/seed_db.sh`
- New: `identity/control-plane/scripts/dev/reset_demo.sh`
- `identity/control-plane/docs/runbooks/demo.md`

Acceptance Criteria

- One command resets the DB to a known baseline and re-seeds data.
- Seed profile selection is documented and produces predictable outcomes.

Tests

- Manual script verification.

Verification Steps

- Run reset, then run a deny/allow demo call and confirm expected outcomes.

---

### Security Remediation: Receipt Signing Hardening (Post-review)

Status: In progress (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Code review identified a trust-boundary weakness: receipt ingest accepted
client-supplied signature metadata without cryptographic verification, and
signing configuration failures could silently degrade integrity guarantees.

Scope

- Make receipt signature metadata server-authoritative for ingest.
- Add required-mode fail-closed behavior for signer initialization/runtime failures.
- Enforce ECDSA P-256 key/algorithm consistency.
- Align OpenAPI/client contracts and security docs.
- Add ADR documenting the in-place `/v1/receipts` security exception.

Acceptance Criteria

- `POST /v1/receipts` rejects `signature_alg`, `signature_kid`, `signature`, `signed_at`.
- Required mode (`UMBRA_RECEIPT_SIGNING_REQUIRED=true`) blocks startup or request path on signing failure.
- `RECEIPT_SIGNING_UNAVAILABLE` is emitted consistently for required-mode failures.
- OpenAPI receipt ingest request schema excludes signature metadata fields.
- ADR is present in `docs/adr/` with rationale, blast radius, and rollback.

Tests

- Receipt package unit tests for policy/curve enforcement.
- Control-plane ingest tests for signature-field rejection and required-mode runtime failure.
- PDP/PEP tests for required-mode init/runtime hard-fail behavior.

Verification Steps

- `go test ./packages/go/receipts ./services/controlplane-api/internal/... ./services/pdp/internal/... ./services/pep-gateway/internal/...`
- `make gen` and re-run contract guards.

---

### Enterprise demo credibility

### UMBRA-0025: mTLS

Status: Completed (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Production deployments should support mTLS for service-to-service and client-to-edge communications.

Historical note (2026-04-27)

- This ticket captured the March 2026 planning snapshot.
- The active mTLS guidance has since moved to edge certificate validation plus trusted token handoff for auth-enabled UI/API traffic.

Scope

- Define where mTLS terminates (ingress, service mesh, or gateway).
- Document required certificate issuance/rotation assumptions.
- Map certificate-authenticated identity into a trusted handoff model for Umbra.
- Provide a reference config (Nginx or Envoy) that:
  - validates client certs
  - hands trusted identity to Umbra
  - forwards to UI/API
- Add a minimal local demo path (optional) showing simulated identity in a clearly non-production flow.

Likely files

- `identity/control-plane/docs/how-to-develop.md` (overview)
- `identity/control-plane/docs/runbooks/` (deploy note)
- New: `identity/control-plane/docs/security/mtls.md`

Acceptance Criteria

- Clear deployment guidance for mTLS in front of UI/API.
- Example config with placeholder cert paths.
- Explicit guidance for how validated edge identity is handed to Umbra safely.

Security note

- Do not embed private keys in repo.
- Document rotation and revocation expectations.

Agent plan

- Draft `docs/security/mtls.md` with architecture + config.
- Add short summary link in `how-to-develop.md`.
- Validate with platform/security owner.
- Preserve request/trace headers end-to-end.

Implementation Notes (2026-03-03)

- Added `identity/control-plane/docs/security/mtls.md` with:
  - production edge-termination recommendation
  - cert lifecycle expectations (high-level)
  - a then-current cert/header mapping sketch for local planning
  - illustrative ingress guidance with placeholder cert paths
  - explicit preservation of `x-umbra-request-id` and `traceparent`
- Added `identity/control-plane/docs/runbooks/mtls_deploy_note.md` with validation checklist and rollback posture.
- Linked mTLS docs from:
  - `identity/control-plane/docs/how-to-develop.md`
  - `identity/control-plane/docs/README.md`
  - `identity/control-plane/README.md`

Current-state clarification (2026-04-27)

- Active docs now describe the hardened posture: auth-enabled UI/API traffic uses verified JWT claims, and mTLS/certificate-auth environments should hand off identity through a short-lived token or trusted auth broker.

Local Verification Run

- `test -f identity/control-plane/docs/security/mtls.md`
- `test -f identity/control-plane/docs/runbooks/mtls_deploy_note.md`
- `rg -n "x-umbra-user|x-umbra-roles|x-umbra-tenant-id|ssl_verify_client|traceparent" identity/control-plane/docs/security/mtls.md`

GitHub Update Draft

```markdown
Ticket: UMBRA-0025 — mTLS
Status: Done

Summary

- Added `docs/security/mtls.md` defining the edge mTLS pattern and trust assumptions.
- Added illustrative ingress guidance with placeholder cert paths for planning and validation.
- Added `docs/runbooks/mtls_deploy_note.md` with deployment validation checklist and rollback posture.
- Linked mTLS docs from `docs/how-to-develop.md`, docs index, and project README.

Verification

- mTLS doc includes client cert validation and trusted identity handoff guidance.
- Reference guidance includes trace/request header preservation expectations.
- Runbook note includes positive/negative validation and rollback guidance.
```

---

### UMBRA-0031: Receipt Signing Implementation

Status: Completed (local, 2026-03-03)  
Author: TBD  
Opened: TBD

Why  
Receipt signing in services is required to verify integrity end-to-end.

Scope

- Implement signer + verifier components in services.
- Wire signing to receipt ingest and storage.
- Add automated verification tests using a placeholder key (non-KMS) for local validation.

Likely files

- `identity/control-plane/services/controlplane-api/*`
- `identity/control-plane/services/pdp/*`
- `identity/control-plane/services/pep-gateway/*`
- `identity/control-plane/packages/go/receipts/*`

Acceptance Criteria

- Receipts are signed on ingest and verifiable via stored signature metadata.
- Verification tests run locally without KMS.

Tests

- Unit tests for signing + verification.
- Integration tests covering signed receipts.

Verification Steps

- Run tests and confirm signatures validate for decision + invocation receipts.

Implementation Notes (2026-03-03)

- Added signer/verification primitives in `identity/control-plane/packages/go/receipts/signing.go`:
  - `ECDSA_P256_SHA256` signer
  - signature metadata validation
  - hash signature verification helpers
  - env-gated signer loading
- Wired signing into receipt writes for:
  - `services/controlplane-api` receipt ingest
  - `services/pdp` decision receipts
  - `services/pep-gateway` invocation receipts
- Updated receipt list/export data paths to include signature metadata fields.
- Updated CSV export format with:
  - `signature_alg`
  - `signature_kid`
  - `signature`
  - `signed_at`
- Updated `identity/control-plane/docs/security/receipt_signing.md` to reflect implemented local signing mode.

Local Verification Run

- `go test ./packages/go/receipts ./services/controlplane-api/internal/... ./services/pdp/internal/... ./services/pep-gateway/internal/...`
- New integration tests verify signed decision/invocation receipts and cryptographic verification with placeholder keys.

GitHub Update Draft

```markdown
Ticket: UMBRA-0031 — Receipt Signing Implementation
Status: Done

Summary

- Implemented ECDSA P-256 signer/verify components in shared receipts package.
- Added env-gated local signing support and wired it into controlplane receipt ingest, PDP decision receipts, and PEP invocation receipts.
- Persisted signature metadata (`signature_alg`, `signature_kid`, `signature`, `signed_at`) in receipt records and exports.
- Added unit and integration tests validating signature generation and verification using placeholder keys.

Verification

- Targeted test suite passed:
  - `go test ./packages/go/receipts ./services/controlplane-api/internal/... ./services/pdp/internal/... ./services/pep-gateway/internal/...`
- Integration tests confirm signed receipts validate cryptographically for both decision and invocation flows.
```

---

## Batch Review (5 Tickets Closed)

Closed in this batch:

- UMBRA-0032
- UMBRA-0034
- UMBRA-0033
- UMBRA-0025
- UMBRA-0031

Current state analysis:

- Demo reliability is significantly improved (one-command bring-up + deterministic checks).
- Evaluation and trace runbooks now provide repeatable, evidence-style validation.
- Enterprise credibility is improved with documented mTLS edge pattern and current token-based auth guidance in the active docs.
- Receipt integrity now includes actual service-level cryptographic signing (local placeholder mode) with tests.

Recommended next 5 tickets:

1. UMBRA-0036 — Control-plane auth hardening and production packaging
2. UMBRA-0037 — Multi-Tenant Isolation (RLS + Tests)
3. UMBRA-0038 — KMS-Backed Receipt Signing (Prod)
4. UMBRA-0039 — Reliability + SLO Pack
5. UMBRA-0040 — Performance + Query Scaling

## Scale readiness (next wave)

### UMBRA-0036: OIDC/AuthN Integration + Server-Side AuthZ

Status: In progress (local, 2026-04-27)  
Author: TBD  
Opened: TBD

Why  
Header-based identity is not safe at scale; roles and tenant context must be derived from trusted auth sources.

Scope

- Add OIDC authentication middleware for UI/API.
- Derive tenant/user/roles from verified claims and enforce them server-side.
- Enforce role/tenant checks server-side in controlplane endpoints.
- Add negative tests for missing/invalid claims.

Likely files

- `identity/control-plane/services/controlplane-api/internal/http-api/*`
- `identity/control-plane/ui/app/api/auth/session/*`
- `identity/control-plane/docs/security/*`

Acceptance Criteria

- Requests without valid auth are rejected with a standard error envelope.
- Roles/tenant are derived from claims and validated server-side.
- UI continues to function with dev auth overrides documented.

Tests

- Integration tests for auth middleware + role gating.

Verification Steps

- Hit an admin endpoint without auth -> 401.
- Hit with insufficient role -> 403.

Implementation Notes (2026-04-27)

- Existing `controlplane-api` JWT auth path was hardened rather than replaced:
  - auth-enabled requests derive tenant/user/roles from verified HS256 bearer token claims
  - role normalization now accepts `roles`, `groups`, `realm_access.roles`, and `resource_access.umbra.roles`
  - claim-derived tenant overrides any client-supplied tenant header when auth is enabled
- UI auth path now aligns with the server trust boundary:
  - `ui/app/api/auth/session` verifies the bearer token instead of trusting forwarded browser identity headers
  - `ui/app/api/controlplane/[...path]` strips spoofable identity headers in auth mode and forwards `Authorization: Bearer`
  - UI now supports provider-capable login/callback/logout routes with HTTP-only cookie-backed sessions
  - shared auth storage/constants and JWT parsing were centralized under `ui/lib/auth/`
  - optional local dev-token fallback is explicit opt-in only
- `identity/control-plane/docs/how-to-develop.md` was updated to document the local auth-enabled flow and required JWT claims.

Local Verification Run

- `go test ./services/controlplane-api/internal/http-api`
- `pnpm -C identity/control-plane/ui lint`
- `pnpm -C identity/control-plane/ui build`

Remaining To Close

- Production-oriented provider packaging and customer IdP mapping guidance.
- Decide whether PDP/PEP service-to-service auth belongs in this ticket or the next production-hardening cut.
- Add broader integration/e2e coverage for auth-enabled UI/API flows.

---

### UMBRA-0037: Multi-Tenant Isolation (RLS + Tests)

Status: Open  
Author: TBD  
Opened: TBD

Why  
Hard isolation is required to prevent cross-tenant data access at scale.

Scope

- Implement row-level security policies for tenant tables.
- Enforce tenant scoping in all queries.
- Add tests that prove cross-tenant reads/writes are blocked.

Likely files

- `identity/control-plane/migrations/*`
- `identity/control-plane/packages/go/storage/*`
- `identity/control-plane/services/*/internal/storage/*`
- `identity/control-plane/docs/security/rls_plan.md`

Acceptance Criteria

- Cross-tenant access is blocked at the DB layer.
- Tests demonstrate RLS enforcement.

Tests

- Integration tests covering cross-tenant denial cases.

Verification Steps

- Attempt cross-tenant query and confirm it fails.

---

### UMBRA-0038: KMS-Backed Receipt Signing (Prod)

Status: Open  
Author: TBD  
Opened: TBD

Why  
Production-grade integrity requires managed key storage and rotation.

Scope

- Replace placeholder signing keys with KMS-backed keys.
- Implement rotation hooks and key versioning metadata.
- Update verification to support key rollover.

Likely files

- `identity/control-plane/services/*`
- `identity/control-plane/packages/go/receipts/*`
- `identity/control-plane/docs/security/receipt_signing.md`

Acceptance Criteria

- Receipts are signed with KMS keys and verifiable across rotations.
- Key rollover procedure validated in a test or runbook.

Tests

- Integration tests covering key rollover and verification.

Verification Steps

- Rotate key and verify historical receipts still validate.

---

### UMBRA-0039: Reliability + SLO Pack

Status: Open  
Author: TBD  
Opened: TBD

Why  
Operational readiness requires defined SLOs, dashboards, and runbooks.

Scope

- Define SLOs for PDP/PEP latency and error rates.
- Add dashboards and alerting thresholds.
- Document incident runbooks for common failure modes.

Likely files

- `identity/control-plane/docs/runbooks/*`
- `identity/control-plane/docs/architecture/*`
- `identity/control-plane/docs/00_exec_summary.md`

Acceptance Criteria

- SLOs are documented and measurable.
- Runbooks cover PDP unavailable, DB downtime, and trace gaps.

Tests

- N/A (documentation + monitoring configuration).

Verification Steps

- Review dashboards and validate alerts are wired.

---

### UMBRA-0040: Performance + Query Scaling

Status: Open  
Author: TBD  
Opened: TBD

Why  
Receipt query load grows quickly; pagination and indexes must keep the UI responsive.

Scope

- Add pagination and limits to receipt queries.
- Add indexes for common filters.
- Add performance tests for receipt listing and search.

Likely files

- `identity/control-plane/services/controlplane-api/internal/storage/*`
- `identity/control-plane/migrations/*`
- `identity/control-plane/ui/app/receipts/*`

Acceptance Criteria

- Receipt list queries remain fast at scale.
- UI pagination remains stable for large datasets.

Tests

- Load tests for receipt queries.

Verification Steps

- Benchmark receipt list with high row counts and confirm response times.

---

### UMBRA-0041: Data Retention + Deletion Policy (Start Framework)

Status: Open  
Author: TBD  
Opened: TBD

Why  
At scale, audit data needs retention boundaries and deletion workflows for compliance.

Scope

- Define retention windows for receipts and related metadata.
- Add deletion workflows for tenant offboarding.
- Document retention and deletion policies.

Likely files

- `identity/control-plane/docs/security/*`
- `identity/control-plane/docs/runbooks/*`
- `identity/control-plane/services/controlplane-api/internal/storage/*`

Acceptance Criteria

- Retention policy documented and approved.
- Deletion workflow documented and validated.

Tests

- Optional integration test for retention job (if implemented).

Verification Steps

- Review policy and confirm expected records are retained/deleted.

---

### UMBRA-0042: Audit Export + Integration Pipeline

Status: Open  
Author: TBD  
Opened: TBD

Why  
Enterprises need receipts exported into external audit systems (SIEM/S3).

Scope

- Add export destinations (batch or streaming).
- Define export format and delivery guarantees.
- Document integration configuration and limits.

Likely files

- `identity/control-plane/services/controlplane-api/*`
- `identity/control-plane/docs/architecture/*`
- `identity/control-plane/docs/runbooks/*`

Acceptance Criteria

- Export pipeline documented with supported targets.
- Export output is verifiable against receipt schema.

Tests

- Integration test for export output (local target).

Verification Steps

- Trigger an export and validate contents and delivery.

---

### UMBRA-0043: API Versioning + Deprecation Policy

Status: Open  
Author: TBD  
Opened: TBD

Why  
Public APIs need explicit compatibility and deprecation rules to avoid breaking clients.

Scope

- Define versioning strategy and deprecation timelines.
- Update OpenAPI to reflect versioning policy.
- Document compatibility guarantees.

Likely files

- `identity/control-plane/docs/api/*`
- `identity/control-plane/docs/README.md`

Acceptance Criteria

- Versioning and deprecation policy documented.
- OpenAPI includes versioning guidance.

Tests

- Contract tests cover versioned endpoints (if applicable).

Verification Steps

- Review API docs for versioning compliance.

---

### UMBRA-0044: Rate Limiting + Abuse Controls

Status: Open  
Author: TBD  
Opened: TBD

Why  
PDP/PEP endpoints need protection from overload and misuse.

Scope

- Add per-tenant rate limits for PDP/PEP.
- Define response semantics for limit exceeded.
- Document limits and tuning knobs.

Likely files

- `identity/control-plane/services/pdp/*`
- `identity/control-plane/services/pep-gateway/*`
- `identity/control-plane/docs/api/*`

Acceptance Criteria

- Rate limits enforced with consistent errors.
- Limits are configurable and documented.

Tests

- Integration test for rate limit breach.

Verification Steps

- Exceed limits and confirm response + metrics.

---

### UMBRA-0045: Backups + Restore Runbook

Status: Open  
Author: TBD  
Opened: TBD

Why  
Operational reliability requires documented backup and recovery procedures.

Scope

- Document backup cadence and storage expectations.
- Provide restore steps for local/dev and production scenarios.
- Include validation steps after restore.

Likely files

- `identity/control-plane/docs/runbooks/*`
- `identity/control-plane/docs/architecture/*`

Acceptance Criteria

- Backup + restore procedure documented.
- Validation steps clearly defined.

Tests

- Manual runbook verification.

Verification Steps

- Perform a restore and confirm the stack recovers.

## Closed tickets

- UMBRA-0030: Contract Tests + Golden Payloads (#42) — Closed
- UMBRA-0029: Property-Based + Fuzz Testing (#41) — Closed
- UMBRA-0028: Idempotency + Replay Protection Semantics (#40) — Closed
- UMBRA-0027: Canonicalization Test Vectors (#39) — Closed
- UMBRA-0026: Receipt Signing + Rotation Plan (#38) — Closed
- UMBRA-0024: Error Envelope Consistency (#32) — Closed
- UMBRA-0023: E2E Propagation (#31) — Closed
- UMBRA-0022: Typed API (#30) — Closed
- UMBRA-0021: Auth Actions (#29) — Closed
- UMBRA-0020: UX Receipts (#28) — Closed
- UMBRA-0019: Policy Control (#27) — Closed
- UMBRA-0018: UI Demo Reliability (#26) — Closed
- UMBRA-0017: Contract Lock (#18) — Closed
- UMBRA-0016: Rust PEP (#17) — Closed
- UMBRA-0015: Receipt Ingest Endpoint (#16) — Closed
- UMBRA-0014: Security Demo Script (#14) — Closed
- UMBRA-0013: Fail-Closed Behavior (#13) — Closed
- UMBRA-0012: Receipt Export (JSON/CSV) (#12) — Closed
- UMBRA-0011: PDP Contract Tests (#11) — Closed
- UMBRA-0010: Policy CRUD Semantics (#10) — Closed
- UMBRA-0009: MCP Demo Integration (#9) — Closed
- UMBRA-0008: MCP Context Model (#8) — Closed
- UMBRA-0007: MCP Identity Mapping (#7) — Closed
- UMBRA-0006: MCP Adapter Service (#6) — Closed
- UMBRA-0005: Correlation Requirements (#5) — Closed
- UMBRA-0004: Receipt Integrity Validation (Hash Chain) (#4) — Closed
- UMBRA-0003: Observe vs Enforce (#3) — Closed
- UMBRA-0002: Deterministic Local Demo (#2) — Closed
- UMBRA-0001: Server-Side Policy Validation (Control Plane API) (#1) — Closed
