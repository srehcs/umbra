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

## UI standards
- Prefer shared UI components under `identity/control-plane/ui/components/ui/`; add a component there instead of raw elements when standardizing behavior.

## CI/CD defaults
- CI must run Go tests (`go test ./...`) from `identity/control-plane`.
- CI must run UI lint (`pnpm lint`) from `identity/control-plane/ui` with a frozen lockfile.
- Lockfiles are required for UI dependencies; do not bypass `--frozen-lockfile` in CI.
