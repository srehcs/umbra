# Receipt Signing Security Hardening Plan (Historical, Completed Locally)

Status: Completed locally on 2026-03-03.

This file is kept as the implementation plan that drove the receipt-signing hardening work. It is no longer the active branch focus; current active work has moved to `UMBRA-0036` and later tickets in `tickets.md`.

## Summary
1. Resolve all three confirmed risks from review:
2. Block forged signature metadata via `POST /v1/receipts` (server-authoritative signatures only).
3. Add required-mode fail-closed behavior for signing config/runtime failures.
4. Enforce true ECDSA P-256 key/algorithm consistency.
5. Deliver in one cohesive PR with ADR security exception, updated contracts/docs, and full regression coverage.
6. Keep SaaS pivot posture intact by tightening trust boundaries, preserving auditability, and avoiding secret exposure.

## Important Public API / Interface / Type Changes
1. `POST /v1/receipts` request contract:
2. Remove `signature_alg`, `signature_kid`, `signature`, `signed_at` from ingest request schema in `docs/api/openapi.yaml`.
3. Runtime behavior: any supplied signature metadata is rejected with `400 RECEIPT_INVALID` (strict now).
4. Export/list surfaces keep signature fields unchanged (`/v1/receipts`, `/v1/receipts/export` response data remains additive/compatible).
5. New runtime config:
6. Add `UMBRA_RECEIPT_SIGNING_REQUIRED` (boolean, default `false`).
7. Semantics: if `required=true`, signer init or signing failure is fail-closed.
8. Internal service interface updates:
9. `Router(logger)` for `controlplane-api`, `pdp`, and `pep-gateway` changes to return `(http.Handler, error)` so startup can fail deterministically.
10. `registerV0` in the same three services returns `error` instead of silent degrade.
11. Shared protocol:
12. Add `RECEIPT_SIGNING_UNAVAILABLE` to `packages/go/protocol/error_codes.go` and use it consistently for signing hard-fail responses.

## Governance and Compliance Steps
1. Add ADR in `docs/adr/` documenting the security exception for in-place `/v1/receipts` hardening (no `/v2`), including rationale, blast radius, and rollback.
2. Update `demo-readiness-two-week-plan.md` and `tickets.md` with a dedicated remediation item and closure criteria.
3. Ensure docs remain compliant with `SECURITY.md` public-doc policy:
4. Keep placeholders only; no real key IDs, cert paths, tenant identifiers, or operational secret detail.

## Implementation Workstreams (Decision-Complete)

### 1) Shared signing primitives and policy
1. In `packages/go/receipts/signing.go`:
2. Add required-mode env parsing for `UMBRA_RECEIPT_SIGNING_REQUIRED`.
3. Rule: `required=true` with `enabled=false` is invalid and returns init error.
4. Add explicit P-256 curve enforcement in `NewECDSAP256Signer`.
5. Add sentinel error for signing unavailability, used by services for precise error mapping.
6. Keep `NewSignerFromEnv` compatibility wrapper if needed, but route services through policy-aware loader.

### 2) Service startup fail-closed wiring
1. Update these routers to return error:  
   `services/controlplane-api/internal/http-api/router.go`  
   `services/pdp/internal/http-api/router.go`  
   `services/pep-gateway/internal/http-api/router.go`
2. Update corresponding `cmd/server/main.go` files to exit non-zero when router init fails.
3. Update each `registerV0` to:
4. Initialize signer with policy-aware loader.
5. Fail startup if `required=true` and signer init fails.
6. Allow unsigned mode only when `required=false`.

### 3) Control-plane ingest trust-boundary hardening
1. In `services/controlplane-api/internal/http-api/v0.go`:
2. Remove signature metadata from accepted ingest semantics.
3. Enforce explicit rejection when signature fields are present in incoming payload.
4. Remove caller-provided signature parsing and pass-through path.
5. Map signing failures to `503 + RECEIPT_SIGNING_UNAVAILABLE` when required mode is active.
6. In `services/controlplane-api/internal/storage/storage.go`:
7. Remove `providedSig` parameter from idempotent insert methods.
8. Signature metadata is generated only from server signer when available.
9. Wrap signing failures with shared sentinel error for upstream handling.

### 4) PDP/PEP runtime fail-closed behavior
1. `services/pdp/internal/http-api/v0.go`:
2. Make `writeDecisionReceipt` return error.
3. If signing fails in required mode, return `503 RECEIPT_SIGNING_UNAVAILABLE` from `/v1/decision`.
4. `services/pep-gateway/internal/http-api/v0.go`:
5. Make `writeInvocationReceipt` return error.
6. In required mode, translate signing failure to fail-closed response (`503 RECEIPT_SIGNING_UNAVAILABLE`).
7. For proxy response path, return error from `ModifyResponse` and map via `ErrorHandler` to the same structured 503 envelope.
8. `services/pdp/internal/storage/storage.go` and `services/pep-gateway/internal/storage/storage.go`:
9. Wrap signer failures with shared sentinel error so handlers can classify accurately.

### 5) Contract and docs alignment
1. Update `docs/api/openapi.yaml`:
2. Remove signature fields from `ReceiptIngestBase` request schema.
3. Keep signature fields in export/list record schemas.
4. Regenerate `packages/contracts/openapi.ts` using existing generation flow.
5. Update `docs/security/receipt_signing.md` to state:
6. Signature metadata is server-managed only.
7. New required-mode behavior and fail-closed guarantees.
8. Update `identity/control-plane/docs/how-to-develop.md` with new env var and strict ingest note.

## Test Cases and Scenarios

### Unit tests
1. `packages/go/receipts/signing_test.go`:
2. Required/enabled env matrix validation.
3. Reject non-P256 ECDSA keys.
4. Existing sign/verify happy path remains green.

### Control-plane API tests
1. Add/extend integration tests in `services/controlplane-api/internal/http-api/integration_test.go`:
2. Ingest with signature fields returns `400 RECEIPT_INVALID`.
3. Ingest with signer configured stores server-generated signature metadata.
4. Required-mode signer init failure causes router init failure (startup block behavior).
5. Required-mode runtime signer failure returns `503 RECEIPT_SIGNING_UNAVAILABLE`.

### PDP tests
1. Extend `services/pdp/internal/http-api/integration_test.go`:
2. Required-mode signer init failure blocks startup.
3. Required-mode signing failure during decision receipt write returns 503 with stable error envelope.

### PEP tests
1. Extend `services/pep-gateway/internal/http-api/integration_test.go` and/or `v0_test.go`:
2. Required-mode signer init failure blocks startup.
3. Required-mode invocation receipt signing failure returns structured 503 (including request/trace correlation IDs).

### Contract/OpenAPI checks
1. Update contract expectations/goldens where behavior changed.
2. Run existing OpenAPI and contract guard tests after schema changes.
3. Run targeted suite:  
   `go test ./packages/go/receipts ./services/controlplane-api/internal/... ./services/pdp/internal/... ./services/pep-gateway/internal/...`
4. Run `make gen` and re-run relevant tests to ensure generated types are in sync.

## Rollout and Safety
1. Single PR rollout with this order:
2. Shared signing package and sentinel errors.
3. Startup fail-closed plumbing.
4. Ingest hardening and storage signature-source lock.
5. PDP/PEP runtime fail-closed handling.
6. OpenAPI/contracts/docs/ADR updates.
7. Verification pass and final check against `RULES.md` DoD.
8. Rollback plan (document in PR):
9. Disable `UMBRA_RECEIPT_SIGNING_REQUIRED` to restore fail-open behavior temporarily in non-prod only.
10. Revert strict ingest rejection only via follow-up ADR decision.

## Assumptions and Defaults Chosen
1. Strict enforcement is immediate (no grace period).
2. `/v1/receipts` remains the endpoint; an ADR explicitly approves this security hardening exception.
3. `UMBRA_RECEIPT_SIGNING_REQUIRED` defaults to `false`; production should set it to `true`.
4. Signature authority is server-only; external signature submission is not trusted.
5. No KMS migration in this patch; this is local-key hardening with a clear path to future KMS-backed rollout.
