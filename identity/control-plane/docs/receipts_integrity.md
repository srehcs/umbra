# Receipt Integrity (Hash Chain)

Receipts are append-only audit records stored in Postgres. Each receipt includes a `prev_hash`
that points to the previous receipt in the chain and a `hash` computed over canonicalized
content plus that `prev_hash`.

## Canonicalization rules

Canonicalization for hashing uses `encoding/json` on Go structs with stable field order. Do not
use maps in receipt bodies unless they are pre-normalized, as Go map iteration is randomized.

Receipt hashing uses only the receipt body and the `prev_hash`:

1. Serialize the receipt body to canonical JSON bytes.
2. Compute `hash = SHA256(prev_hash || body_json)` where `prev_hash` is the hex string (or empty).

Notes:
- The chain is per receipt kind: decision receipts chain independently from invocation receipts.
- The first receipt in a chain uses an empty `prev_hash`.
- For windowed verification (last N receipts), the first receipt in the window is treated as an anchor
  and does not need `prev_hash` to be empty.

## Verification

Verification recomputes each receipt hash and ensures `prev_hash` matches the previous receipt in the
ordered chain (oldest to newest). Failures are reported with:

- `HASH_MISMATCH` when `hash` does not match the recomputed value.
- `MISSING_LINK` when a chain link is broken (e.g., deleted receipt).
- `OUT_OF_ORDER` when a receipt references a later hash in the ordered chain.

This is signing-ready: tables already include optional signature fields (`signature_alg`,
`signature_kid`, `signature`, `signed_at`). See `docs/security/receipt_signing.md` for the
signing and rotation plan.

## Known issue: HASH_MISMATCH from JSONB reordering

When receipts are stored, the hash is computed from canonical JSON bytes produced by Go struct
marshaling (field order is stable). During verification, the current implementation reads
`body_json` back from Postgres as JSONB text and hashes those bytes. JSONB does not preserve key
order, so the byte sequence can differ from the original canonical JSON even if the semantic
content is identical. This can trigger `HASH_MISMATCH` even on a fresh database.

Root cause:
- Hashing uses canonical JSON bytes (Go struct field order).
- Verification uses JSONB text (Postgres key order).
- Different byte order → different hash.

Implication:
- `HASH_MISMATCH` may appear even without tampering when JSONB reorders keys.

Mitigation:
- Store the canonical JSON bytes used for hashing in `body_canonical` and verify against that column when present. This preserves the exact byte sequence used at hash time.

Reset note:
- The dev Postgres uses a bind mount at `identity/control-plane/deployments/local/.data/postgres`. To fully reset receipts for verification, stop the stack and delete that directory before reseeding.

Example reset commands:
```bash
cd identity/control-plane
docker compose -f deployments/docker-compose.yml down
rm -rf deployments/local/.data/postgres
make dev
make seed
```
