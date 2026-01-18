# Receipt Signing and Rotation (V0)

Receipts are audit artifacts. This document defines the signing workflow and
rotation process without storing secrets in the repo.

## Signature fields
Receipt schema supports optional signature metadata:
- `signature_alg`
- `signature_kid`
- `signature`
- `signed_at`

These fields appear in receipt ingest/export payloads and are stored alongside
the receipt hash chain fields.

## Signing flow (KMS-backed)
1. Serialize receipt body to canonical JSON bytes.
2. Compute `receipt_hash = SHA256(prev_hash || body_json)`.
3. Call KMS `Sign` with the hash bytes (or hash digest, per KMS policy).
4. Store the signature metadata:
   - `signature_alg`: e.g., `RSASSA_PSS_SHA_256` or `ECDSA_P256_SHA256`
   - `signature_kid`: key identifier/ARN
   - `signature`: base64-encoded signature
   - `signed_at`: UTC timestamp

## Verification flow
1. Recompute `receipt_hash` from stored `body_canonical` and `prev_hash`.
2. Resolve the signing key by `signature_kid`.
3. Verify the signature over the hash bytes.
4. If verification fails, treat as audit integrity failure.

## Telemetry (recommended)
- Metrics:
  - `receipt_signature_verify_total{result}` with `result=ok|fail`
  - `receipt_signature_sign_total{result}` with `result=ok|fail`
- Logs (structured):
  - `receipt_id`, `request_id`, `decision_id`, `signature_kid`, `signature_alg`, `result`

## Verification test example (placeholder key, non-normative)
This is a local-only test harness example. It should never ship with real keys:

```text
1) Load a placeholder public key.
2) Compute receipt_hash from canonical bytes.
3) Verify signature (base64) against receipt_hash.
```

## Rotation schedule (suggested defaults)
- **Active key:** 30 days
- **Overlap (grace) period:** 60 days (verify old signatures)
- **Retention:** retain old keys until all receipts signed by them age out

Note: cadence and examples here are non-normative and for planning only; they do not imply
production keys or hardcoded behavior.

## Rotation decision table
| Key age | Sign | Verify | Action |
| --- | --- | --- | --- |
| 0-30 days | Yes | Yes | Active key |
| 31-90 days | No | Yes | Grace period |
| >90 days | No | No | Retire key after retention window |

## Key rollover procedure
1. Generate a new KMS key (or key version) and set it as active for signing.
2. Keep previous key available for verification during the grace period.
3. Update `signature_kid` on new receipts only.
4. After grace period, revoke old key for signing; keep read-only for verification
   until data retention policies allow removal.

## Rollback (mis-configured key)
1. Disable the new key for signing.
2. Revert the signer to the last known-good `signature_kid`.
3. Reissue signatures for any receipts created during the fault window.

## Compliance checklist
- No private key material stored in repo.
- `signature_kid` maps to KMS-managed keys only.
- Rotation cadence documented and enforced.
- Verification uses canonical JSON bytes (`body_canonical`).
- Schema guard script passes (`scripts/dev/verify_signature_schema.sh`).

## Future work
- Implement signer/verify components in services and wire to receipt ingest.
- Add automated verification tests with a placeholder key (non-KMS) for local validation.
