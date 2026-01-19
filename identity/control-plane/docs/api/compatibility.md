# API Compatibility Rules (V0)

These rules prevent contract drift between services, OpenAPI, and generated clients.

## Compatibility policy
- **Breaking changes** require a new versioned endpoint (e.g. `/v2/decision`) and an ADR.
- **Additive changes** are allowed when fields are optional and documented in OpenAPI.
- **Required fields** cannot be removed or have their meaning changed in-place.
- **Enums** may only gain new values in a backward-compatible way.
- **Response envelopes** must keep `error.code`, `error.message`, and `request_id` stable.

## Contract testing
- Golden payloads live in `docs/test_vectors/contracts/`.
- Contract tests must fail on incompatible response shape changes.
- Update OpenAPI and vectors together to keep generated clients aligned.

## OpenAPI compatibility guard
- Baseline expectations live in `docs/test_vectors/contracts/openapi_baseline.json`.
- The guard fails if required fields are removed or if response schemas/content types drift.
- Update the baseline only when an ADR approves a breaking change.
