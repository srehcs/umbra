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

## Signing flow (managed signing service)
1. Serialize receipt body to canonical JSON bytes.
2. Compute `receipt_hash = SHA256(prev_hash || body_json)`.
3. Call a managed signing service with the hash bytes (or digest).
4. Store the signature metadata:
   - `signature_alg`: e.g., `RSASSA_PSS_SHA_256` or `ECDSA_P256_SHA256`
   - `signature_kid`: opaque key reference identifier
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
  - `receipt_id`, `request_id`, `decision_id`, `signature_alg`, `result`

## Verification test example (placeholder key, non-normative)
This is a local-only test harness example. It should never ship with real keys:

```text
1) Load a placeholder public key.
2) Compute receipt_hash from canonical bytes.
3) Verify signature (base64) against receipt_hash.
```

## Rotation policy (high-level)
- Rotate signing keys on a periodic cadence defined by internal security policy.
- Maintain an overlap window where old keys remain available for verification.
- Retire keys only after verification requirements and data-retention rules are met.
- Keep detailed runbooks, intervals, and key inventories in internal-only docs.

## Compliance checklist
- No private key material stored in repo.
- `signature_kid` is an opaque reference, not a provider resource name or ARN.
- Rotation cadence documented internally and enforced.
- Verification uses canonical JSON bytes (`body_canonical`).
- Schema guard script passes (`scripts/dev/verify_signature_schema.sh`).

## Future work
- Implement signer/verify components in services and wire to receipt ingest.
- Add automated verification tests with a placeholder key for local validation.
