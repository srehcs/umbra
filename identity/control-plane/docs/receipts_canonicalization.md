# Receipt Canonicalization (V0)

Deterministic hashing depends on stable byte serialization. In V0 we use Go's
`encoding/json` marshaling on structs to produce canonical JSON bytes.

## Rules
- Canonicalization uses Go structs with explicit field order; maps are forbidden.
- Do not use floats in receipt bodies (avoid 1 vs 1.0 ambiguity).
- Use `omitempty` for optional fields; zero values must be omitted.
- Arrays preserve order; do not reorder elements for hashing.
- Only ASCII keys and values are allowed in receipts for V0.

Non-compliance invalidates hashes and is a test failure.

## Test vectors
Canonicalization vectors live under:
`identity/control-plane/docs/test_vectors/canonicalization/`

Each vector includes:
- `type`: identifies the struct used for canonicalization.
- `input`: JSON payload to unmarshal into the struct.
- `canonical`: expected canonical JSON bytes (no whitespace).
- `hash`: SHA256 of `canonical` bytes (hex).

Vectors are validated in Go tests under:
`identity/control-plane/packages/go/receipts/`.
