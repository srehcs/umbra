# Agent Context — Umbra Identity Control Plane (V0-C)

Umbra is an enterprise **security control layer** for AI agents. It sits at the tool boundary (MCP-first):
- **PEP** intercepts tool invocations
- **PDP** returns allow/deny (and later obligations)
- **Receipts** record what happened (hash-chained, signing-ready) for audit/IR
- **UI** (Next.js + ShadCN) lets operators manage policies/tools and inspect receipts

## Repo boundaries (do not violate)
- Active build: `umbra/identity/control-plane/`
- Docs live only in: `umbra/identity/control-plane/docs/`
- Engineering rules: `umbra/RULES.md` (non-negotiable)

## Local dev entrypoints
- `umbra/identity/control-plane/Makefile` is the source of truth
- Local stack: `umbra/identity/control-plane/deployments/docker-compose.yml`

## Key contracts
- OpenAPI: `umbra/identity/control-plane/docs/api/openapi.yaml`
- Go protocol types: `umbra/identity/control-plane/packages/go/protocol/*`
- Receipts + hashing: `umbra/identity/control-plane/packages/go/receipts/*`
- OTel helpers: `umbra/identity/control-plane/packages/go/otel/*`

## Quality bar
- Type-safe code, no placeholder content, no secrets in logs/receipts
- Bounded retries/timeouts; safe failure modes (fail-closed where required)
- Tests for non-trivial logic + integration tests for cross-service flows
