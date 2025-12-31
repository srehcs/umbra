# Umbra Agent Guide

## Build, Test, Lint

From `identity/control-plane/`:
```bash
make bootstrap         # Install deps
make fmt              # Format (gofmt + prettier)
make lint             # Lint checks (go vet + eslint)
make test             # Run Go tests
make test -B          # Run single test: go test ./path -run TestName
make gen              # Generate (OpenAPI → TS stubs)
make dev              # Local stack: docker compose up + seed
make seed             # Load seed data into postgres
```
Note: If `make dev` fails with `bind: address already in use` on port 5432, a local Postgres is running. Stop it (e.g., `/Library/PostgreSQL/14/bin/pg_ctl -D /Library/PostgreSQL/14/data stop -m fast`) or remap the docker compose Postgres port.
Note: When `make dev` succeeds, it prints local endpoints (UI :3000, API :8080, PDP :8081, PEP :8082) and seeded tenant IDs; use those IDs with the `x-umbra-tenant-id` header.
Example API calls (replace tenant ID):
```bash
curl -s -H "x-umbra-tenant-id: <tenant-id>" http://localhost:8080/v1/tools
curl -s -H "x-umbra-tenant-id: <tenant-id>" http://localhost:8080/v1/policies
```

## Architecture

**Umbra** is an enterprise security layer for AI agents: PDP (Policy Decision Point), PEP (Policy Enforcement Point), and audit receipts.

**Key Services** (from `identity/control-plane/services/`):
- `controlplane-api`: Admin API (policies, tools, actor roles) — port 8080
- `pdp`: Policy Decision Point (evaluates allow/deny) — port 8081
- `pep-gateway`: Policy Enforcement Point (intercepts & enforces MCP tool calls) — port 8082
- `ui`: Next.js control plane UI (manage policies, view receipts) — port 3000

**Packages** (from `identity/control-plane/packages/go/`):
- `protocol/`: Decision request/response types
- `policy/`: ABAC v0 policy evaluation engine
- `receipts/`: Hash-chained audit trail generation
- `storage/`: PostgreSQL queries
- `otel/`: OpenTelemetry tracing

**Database**: PostgreSQL (migrations in `migrations/`)

## Code Style

- **Go**: Use `gofmt`, follow stdlib idioms, include OpenTelemetry context propagation
- **TypeScript/React**: Use Next.js 14, ShadCN components, Tailwind, Zod for validation
- **Imports**: Keep short, group stdlib → third-party → internal
- **Naming**: Descriptive (no abbreviations except common ones: ctx, id, err)
- **Error handling**: Return errors; no silent failures. Standardize to `{ error: { code, message }, request_id }`
- **Serialization**: Deterministic JSON for receipts; use canonical field order
- **Types**: Strong typing; Zod schemas for API boundaries
- **Secrets**: Never log or store sensitive data; redact in receipts

## Rules

See `RULES.md` for engineering standards (secure SDLC, auditability, PR discipline, no secrets in git).
