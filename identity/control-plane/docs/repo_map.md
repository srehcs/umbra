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
- `services/pdp/`
  - decision endpoint (`POST /v1/decision`)
  - policy evaluation (ABAC V0), default deny
- `services/pep-gateway/`
  - enforcement entrypoint for demo (MCP/CLI expansion path)
  - calls PDP then records invocation receipt

### Packages
- `packages/go/policy/`
  - ABAC V0 evaluator and policy types
- `packages/go/receipts/`
  - canonical JSON, hashing and chain utilities
  - signing-ready extension points
- `packages/go/storage/`
  - DB connection helpers

### UI
- `ui/`
  - Next.js + ShadCN console (tools, policies, receipts)
  - strict TypeScript types for API contracts

### Docs
- `docs/`
  - `api/openapi.yaml` — API contract
  - `/docs/adr/` — architectural decisions (centralized)
  - `architecture/` — C4 diagrams (context/containers/components)
  - `security/` — threat model and OIDC “path to yes”
  - `examples/` — curl scripts for demo flows

### Deployments
- `deployments/docker-compose/` — local stack for demo and development

## “Where to change X?”
- policy semantics: `packages/go/policy/` + `services/pdp/`
- receipts integrity: `packages/go/receipts/` + service receipt writers
- UI display: `ui/app/*`
- API contract: `docs/api/openapi.yaml`
