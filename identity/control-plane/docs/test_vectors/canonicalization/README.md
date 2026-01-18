# Canonicalization Vectors

These vectors define the canonical JSON byte sequences used for receipt hashing.
They are intended to be used across languages to verify deterministic output.

Files:
- `vectors.json`: inputs and expected canonical JSON (plus SHA256 hash).

How to use:
1. Unmarshal the `input` JSON into the corresponding struct type.
2. Serialize to canonical JSON bytes (no whitespace).
3. Compare the bytes to `canonical` and hash to `hash`.

If your implementation produces different bytes, receipt hashes will diverge.
