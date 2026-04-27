# Two-Week Plan (Demo Readiness)

Owner: Umbra team. Scope: demo readiness plus enterprise credibility work.
Last synced: 2026-03-03 against `https://github.com/srehcs/umbra/issues` open issues.

Status note (2026-04-27):

- This document is now a historical record of the demo-readiness batch.
- That batch is complete locally.
- Active follow-on work has moved to `UMBRA-0036` and later tickets in `tickets.md`.
- Historical auth/header language here is retained only where it describes auth-disabled local demo flows; current auth-enabled behavior is documented in the active control-plane docs.

## Ticket relevance

This plan focuses on a 2-week subset of currently open tickets (see `tickets.md` for the full open list):

- UMBRA-0032: Operational Readiness Pack (One-Command Bring-up)
- UMBRA-0034: Evaluation Walkthrough + Expected Output Pack
- UMBRA-0033: Trace + Observability Verification Runbook
- UMBRA-0025: mTLS
- UMBRA-0031: Receipt Signing Implementation

## Goals for this 2-week cut

1. **Operational bring-up**: one command to start, seed, and verify the stack.
2. **Evaluation walkthrough**: concise, reliable steps with expected outputs.
3. **Trace proof**: deterministic trace/receipt correlation runbook.
4. **Enterprise credibility**: mTLS deployment plan + receipt signing implementation.

## Dependencies (critical path)

- UMBRA-0032 -> UMBRA-0034 (walkthrough uses the one-command bring-up flow).
- UMBRA-0032 -> UMBRA-0033 (trace runbook assumes a healthy stack).

## Recommended ticket order

1. UMBRA-0032 (Operational Readiness Pack (One-Command Bring-up))
2. UMBRA-0034 (Evaluation Walkthrough + Expected Output Pack)
3. UMBRA-0033 (Trace + Observability Verification Runbook)
4. UMBRA-0025 (mTLS)
5. UMBRA-0031 (Receipt Signing Implementation)

## Remaining work

### UMBRA-0032: Operational Readiness Pack (One-Command Bring-up)

Goal: one-command bring-up with health and verification steps.
Status: Completed (local, 2026-03-03)
Completed:

- Added `make demo` target wired to `scripts/dev/demo_start.sh`.
- Added one-command flow that brings up services, waits, seeds, runs `demo-check`, and prints tenant IDs + verify curl commands.
- Updated demo docs to reference one-command flow and clarify enforce vs observe deny behavior.
  Verification (completed):
- `make demo`
- `bash -n scripts/dev/demo_start.sh`
- `docker compose -f deployments/docker-compose.yml config`
- `curl -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/demo` -> `200`, `hello-from-upstream`
- `curl -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/secret` -> `403`, `POLICY_DENIED`
- `curl -X POST -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100"` -> `ok=true`

GitHub Update Draft:

- Ready-to-post close note added under `UMBRA-0032` in `tickets.md` (`GitHub Update Draft` block).

### UMBRA-0034: Evaluation Walkthrough + Expected Output Pack

Goal: concise, repeatable walkthrough with expected results.
Status: Completed (local, 2026-03-03)
Completed:

- Added `identity/control-plane/docs/runbooks/demo_walkthrough.md` with a 6-step evaluator flow:
  baseline policy/tool -> allow -> deny -> receipts correlation -> integrity verify -> UI proof.
- Included expected JSON snippets for policy/tool list, deny envelope, receipts correlation, and verify output.
- Added screenshot placeholders for UI evidence capture without embedding secrets.
- Linked walkthrough from `identity/control-plane/docs/README.md`, `identity/control-plane/README.md`, and `docs/runbooks/demo.md`.
  Verification (completed):
- `make demo`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" http://localhost:8080/v1/tools | jq .`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" http://localhost:8080/v1/policies | jq .`
- `curl -i -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/demo` -> `200`
- `curl -i -H "x-umbra-tenant-id: <TenantA>" http://localhost:8082/tool/secret` -> `403`
- `curl -s -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts?limit=3" | jq .`
- `curl -s -X POST -H "x-umbra-tenant-id: <TenantA>" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100" | jq .` -> `ok=true`
  GitHub Update Draft:
- Ready-to-post close note added under `UMBRA-0034` in `tickets.md` (`GitHub Update Draft` block).

### UMBRA-0033: Trace + Observability Verification Runbook

Goal: show trace/receipt correlation with clear instructions.
Status: Completed (local, 2026-03-03)
Completed:

- Added `identity/control-plane/docs/runbooks/trace_demo.md` with both script-driven and manual correlation flows.
- Added `identity/control-plane/scripts/dev/trace_smoke.sh` to generate a trace, query receipts by `request_id`, and print `trace_id`/`span_id`/`decision_id` plus Jaeger lookup guidance.
- Linked trace runbook from `identity/control-plane/docs/README.md`, `identity/control-plane/README.md`, and `docs/runbooks/demo.md`.
  Verification (completed):
- `bash -n scripts/dev/trace_smoke.sh`
- `TENANT_ID=<TenantA> bash scripts/dev/trace_smoke.sh` -> deny flow: `403`, non-empty `trace_id`, `span_id`, `decision_id`
- `TENANT_ID=<TenantA> TRACE_KIND=allow bash scripts/dev/trace_smoke.sh` -> allow flow: `200`, non-empty `trace_id`, `span_id`, `decision_id`
  GitHub Update Draft:
- Ready-to-post close note added under `UMBRA-0033` in `tickets.md` (`GitHub Update Draft` block).

### UMBRA-0025: mTLS

Goal: document mTLS termination and header mapping.
Status: Completed (local, 2026-03-03)
Completed:

- Added `identity/control-plane/docs/security/mtls.md` with:
  - recommended edge-termination model
  - certificate lifecycle expectations (high-level, no environment secrets)
  - a March 2026 planning sketch for certificate-derived identity handoff
  - illustrative ingress guidance with placeholder cert paths
  - trace/request header preservation notes
- Added `identity/control-plane/docs/runbooks/mtls_deploy_note.md` with validation checklist and rollback posture.
- Added links in `identity/control-plane/docs/how-to-develop.md`, `identity/control-plane/docs/README.md`, and `identity/control-plane/README.md`.
  Verification (completed):
- `test -f docs/security/mtls.md`
- `test -f docs/runbooks/mtls_deploy_note.md`
- `rg -n "x-umbra-user|x-umbra-roles|x-umbra-tenant-id|ssl_verify_client|traceparent" docs/security/mtls.md`
  GitHub Update Draft:
- Ready-to-post close note added under `UMBRA-0025` in `tickets.md` (`GitHub Update Draft` block).

### UMBRA-0031: Receipt Signing Implementation

Goal: implement signer/verifier and tests.
Status: Completed (local, 2026-03-03)
Completed:

- Added ECDSA P-256 signer/verification implementation in `packages/go/receipts/signing.go`.
- Added signer env-loading (`UMBRA_RECEIPT_SIGNING_ENABLED`, `..._KID`, `..._PRIVATE_KEY_PEM`) with safe default disabled behavior.
- Wired signer into:
  - `services/controlplane-api` receipt ingest path
  - `services/pdp` decision receipt writes
  - `services/pep-gateway` invocation receipt writes
- Updated controlplane receipt list/export payload generation to include signature metadata fields.
- Updated CSV export columns to include `signature_alg`, `signature_kid`, `signature`, `signed_at`.
- Updated signing docs in `docs/security/receipt_signing.md` to reflect implemented local placeholder mode.
  Verification (completed):
- `go test ./packages/go/receipts ./services/controlplane-api/internal/... ./services/pdp/internal/... ./services/pep-gateway/internal/...`
- New integration tests validate signed receipt metadata and cryptographic verification for decision + invocation flows.
  GitHub Update Draft:
- Ready-to-post close note added under `UMBRA-0031` in `tickets.md` (`GitHub Update Draft` block).

## Batch Review (5 Tickets Closed)

Closed in this batch:

- UMBRA-0032
- UMBRA-0034
- UMBRA-0033
- UMBRA-0025
- UMBRA-0031

Current development state:

- Demo bring-up and evaluator walkthrough are now one-command and evidence-backed.
- Trace/receipt correlation has a deterministic smoke script and runbook.
- mTLS deployment posture is documented, and the active docs now prefer trusted token handoff for auth-enabled UI/API traffic.
- Receipt signing now exists in controlplane, PDP, and PEP with local placeholder keys and automated tests.

What should come next (next 5):

1. UMBRA-0036 (Control-plane auth hardening): complete provider packaging, auth-enabled coverage, and production ingress alignment.
2. UMBRA-0037 (Multi-tenant isolation / RLS): enforce tenant boundaries at DB policy layer.
3. UMBRA-0038 (KMS-backed signing): replace placeholder keys with production key custody + rollover.
4. UMBRA-0039 (Reliability + SLO pack): formalize operational expectations and incident controls.
5. UMBRA-0040 (Performance + query scaling): keep receipts UX responsive under scale.

## Follow-on status (2026-04-27)

### UMBRA-0036 snapshot

- Status: In progress (local)
- Completed in the current branch:
  - controlplane API auth-enabled requests now derive tenant/user/roles from verified bearer token claims
  - role normalization accepts `roles`, `groups`, `realm_access.roles`, and `resource_access.umbra.roles`
  - auth-enabled UI session/proxy flow no longer trusts browser-supplied identity headers
  - provider-capable UI login/callback/logout routes exist with HTTP-only cookie-backed sessions
  - local developer docs now describe the token-based auth-enabled path
- Still remaining:
  - production-oriented provider packaging and customer IdP mapping guidance
  - any additional service-to-service auth scope not already covered by current tickets
  - broader auth-enabled integration/e2e coverage
