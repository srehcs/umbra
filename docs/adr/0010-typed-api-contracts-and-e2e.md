# ADR-0010: Typed API Contracts and E2E Harness

- Status: accepted
- Date: 2026-01-01
- Owners: @srehcs

## Context
We need compile-time safety between OpenAPI and UI integrations and a deterministic, repeatable E2E workflow. Manual fetch types and ad-hoc tests drift across services and slow down demos.

Constraints:
- Keep UI provider-agnostic and avoid heavy runtime dependencies.
- Preserve local dev workflows (`make dev`, `make seed`).
- Ensure E2E tests do not assume stability; they must control tenant IDs and seed data.

## Decision
- Generate TypeScript types from `docs/api/openapi.yaml` using `openapi-typescript`.
- Use `openapi-fetch` for a typed UI client.
- Commit generated contracts under `identity/control-plane/packages/contracts/`.
- Add Playwright smoke tests with a deterministic seed harness (`make e2e`).

## Alternatives considered
1) Continue hand-rolled types in `ui/lib/types.ts` and manual fetches.
2) Use orval (more generated code, heavier footprint).

## Consequences
- Positive:
  - Breaking API changes surface as TypeScript errors.
  - E2E smoke is reproducible across machines via deterministic seed IDs.
- Negative:
  - Requires regeneration step when OpenAPI changes.
  - E2E introduces Playwright dependency and local setup.
- Follow-ups:
  - Keep OpenAPI and generated contracts in sync via CI (future).
  - Expand E2E coverage beyond smoke if needed.
