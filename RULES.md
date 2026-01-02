# Umbra Engineering Rules (Shared Practices)

This repo assumes you already maintain a canonical `RULES.md` in your Umbra umbrella.
If you copied that file here, keep it authoritative and enforced via code review and CI.

**Non-negotiables**
- Secure SDLC gates
- Observability + receipts (auditability)
- PR discipline + review requirements
- No secrets in git

See: `docs/how-to-develop.md` for the “golden path” workflow used by this scaffold.

## Performance and scalability defaults
- Prefer DB-side filtering, ordering, and pagination; avoid in-memory merges for large datasets.
- Avoid O(n^2) loops; use standard library sort or indexed queries.
- Any new filterable fields must ship with indexes or generated columns.
- Export paths must stream (CSV/JSONL) for large outputs; avoid loading full datasets in memory.
- Free-text search must be explicit about scope; avoid unindexed substring scans on large tables.
- UI fetches must be abortable; cancel stale requests on unmount or filter changes.
- UI should avoid expensive render-time serialization (e.g., JSON.stringify in render); memoize or lazy-render.

## Rust standards
- Prefer safe Rust; no `unsafe` in request paths without explicit justification and isolation.
- Use `clippy` clean builds (`cargo clippy -- -D warnings`) for new Rust services.
- Enforce timeouts and bounded concurrency on outbound calls.
- Avoid blocking in async paths; use `spawn_blocking` for CPU-heavy or blocking I/O.
- Use bounded channels/queues; avoid unbounded buffers without backpressure.
- Avoid logging sensitive data; redact in receipts and logs.

## Go standards
- Keep request handlers thin; move domain logic into package-level services.
- Always propagate `context.Context`; honor cancellations/timeouts on I/O.
- Return typed errors; map to `{ error: { code, message }, request_id }` at boundaries.
- Avoid global mutable state; inject deps for testability.
- Use `sql`/`pgx` with explicit columns; avoid `SELECT *` in query paths.
- Validate input at API edges; keep internal structs trusted and minimal.

## TypeScript standards
- Strict typing on; prefer `unknown` over `any`, and avoid `as` casts unless documented and isolated.
- OpenAPI types are first-class contracts; use Zod for runtime validation where inputs can be untrusted.
- Zod schemas are the contract at boundaries; infer types from schemas and validate runtime inputs.
- Keep API clients centralized; no ad-hoc `fetch` scattered across pages.
- Prefer server-side data fetching when inputs are cacheable or not user-specific; use client fetches when driven by client-only state or interactivity.
- Avoid repeated derived computation in render; memoize heavy transforms.
- Never log secrets or full payloads; redact before emitting telemetry.

## Advanced systems practices
- Deterministic canonicalization with test vectors (cross-language reproducibility).
- Signed receipts with rotation plan (KMS-backed) and signature metadata in schema.
- Idempotency and replay protection via `request_id` (dedupe semantics documented).
- Streaming exports with backpressure and bounded memory (JSONL/CSV).
- Property-based/fuzz tests for parsing, canonicalization, and policy evaluation.
- Contract tests across services with schema versioning and golden payloads.

## UI standards
- Prefer shared UI components under `identity/control-plane/ui/components/ui/`; add a component there instead of raw elements when standardizing behavior.

## CI/CD defaults
- CI must run Go tests (`go test ./...`) from `identity/control-plane`.
- CI must run UI lint (`pnpm lint`) from `identity/control-plane/ui` with a frozen lockfile.
- Lockfiles are required for UI dependencies; do not bypass `--frozen-lockfile` in CI.
