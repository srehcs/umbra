# Repo Map (Identity Control Plane)

## Umbra root
- `RULES.md` — non-negotiable engineering practices
- `identity/` — identity-related docs + projects
- `identity/control-plane/` — the V0-C implementation

## identity/control-plane/
### Services
- `services/controlplane-api/`
  - tool and policy management (CRUD)
  - receipts query API for the UI
  - JWT authn + server-side role/tenant enforcement in `internal/http-api/`
- `services/pdp/`
  - decision endpoint (`POST /v1/decision`)
  - policy evaluation (ABAC V0), default deny
- `services/pep-gateway/`
  - enforcement entrypoint for demo (MCP/CLI expansion path)
  - calls PDP then records invocation receipt
- `services/mcp-adapter/`
  - MCP tool-call enforcement adapter (PEP)
  - calls PDP then forwards to MCP upstream
- `services/mcp-upstream/`
  - demo MCP server for local stack

### Packages
- `packages/go/policy/`
  - ABAC V0 evaluator and policy types
- `packages/go/receipts/`
  - canonical JSON, hashing and chain utilities
  - signing-ready extension points
- `packages/go/storage/`
  - DB connection helpers
- `packages/go/protocol/`
  - shared API request/response types
- `packages/go/testutil/`
  - contract harness and golden matcher helpers for tests
- `packages/contracts/`
  - generated OpenAPI TS types (shared with UI)

### UI
- `ui/`
  - Next.js + ShadCN console (tools, policies, receipts)
  - strict TypeScript types for API contracts
  - generated OpenAPI client types in `ui/contracts/`
  - auth/session helpers in `ui/lib/auth/`
  - control-plane proxy routes in `ui/app/api/`

### Docs
- `docs/`
  - `api/openapi.yaml` — API contract
  - `api/compatibility.md` — schema compatibility guard
  - `api/error_envelope.md` — error model details
  - `/docs/adr/` — architectural decisions (centralized)
  - `architecture/` — C4 diagrams (context/containers/components)
  - `security/` — threat model, receipt signing, and OIDC “path to yes”
  - `test_vectors/` — canonicalization + contract payloads
  - `examples/` — curl scripts for demo flows
  - `idempotency.md` — replay protection and dedupe semantics

### Deployments
- `deployments/docker-compose.yml` — local stack for demo and development

## “Where to change X?”
- policy semantics: `packages/go/policy/` + `services/pdp/`
- receipts integrity: `packages/go/receipts/` + service receipt writers
- UI display: `ui/app/*`
- API contract: `docs/api/openapi.yaml`
