# ADR-0013: Strict Receipt-Signing Ingest Hardening on `/v1/receipts`

- Status: accepted
- Date: 2026-03-03
- Owners: Umbra control-plane team

## Context
Umbra supports signature metadata on stored receipts (`signature_alg`, `signature_kid`,
`signature`, `signed_at`). The original `POST /v1/receipts` behavior allowed clients to
submit these fields. That created a trust-boundary weakness: unsigned or forged metadata
could be persisted when the service-side signer was disabled.

The repository compatibility policy generally requires versioned endpoints for breaking
changes. This hardening intentionally changes request semantics in-place for a security
reason and must ship immediately.

Constraints:
- `RULES.md`: fail-closed defaults for high-risk surfaces and strong auditability.
- `SECURITY.md`: no secrets in repo; public docs remain high-level.
- SaaS pivot readiness: preserve verifiable, server-owned receipt integrity semantics.

## Decision
1. Keep endpoint path as `POST /v1/receipts` (no `/v2` fork for this security fix).
2. Treat signature metadata as server-managed only for ingest.
3. Reject client-supplied `signature_alg`, `signature_kid`, `signature`, `signed_at`
   with `400 RECEIPT_INVALID`.
4. Introduce `UMBRA_RECEIPT_SIGNING_REQUIRED` (default `false`) for fail-closed behavior:
   - If `required=true`, signer initialization and runtime signing failures return
     `RECEIPT_SIGNING_UNAVAILABLE` and block request processing where applicable.
5. Preserve signature metadata in receipt read/export schemas and storage.

## Alternatives considered
1. Add `/v2/receipts` for strict behavior and keep `/v1/receipts` legacy semantics.
   - Rejected: leaves the vulnerable path active and delays hardening.
2. Keep request fields but ignore them silently.
   - Rejected: ambiguous contract and poor operator visibility.
3. Keep accepting client metadata and add optional verification later.
   - Rejected: does not close the immediate integrity gap.

## Consequences
- Positive:
  - Server is the single authority for receipt signatures.
  - Required-mode deployments can enforce fail-closed signing guarantees.
  - Audit semantics are stronger and more SaaS-ready.
- Negative:
  - Breaking request behavior for clients that previously posted signature fields.
  - Requires OpenAPI/client regeneration and downstream notice.
- Follow-ups:
  - Implement KMS-backed signing and key lifecycle workflows.
  - Extend receipt verification endpoints to include signature verification results.
