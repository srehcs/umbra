# Umbra — Agent Identity Control Plane (V0-C)

Umbra’s **Agent Identity Control Plane** is an enterprise-facing control plane that governs **agent/tool invocations**
by enforcing policy at a **Policy Enforcement Point (PEP)**, evaluating requests in a **Policy Decision Point (PDP)**,
brokering **least-privilege credentials** (later), and emitting **tamper-evident receipts** for audit/IR.

## What ships in this scaffold
- Monorepo layout with clear boundaries (services / packages / ui / docs / deployments)
- By-the-book artifacts: ADRs, threat model skeleton, PR/security templates, CODEOWNERS
- Local dev stack (`docker compose up`) including Postgres + Redis + OTel collector + Jaeger
- Skeleton services:
  - `controlplane-api` (policy/tool registry, audit query)
  - `pdp` (decision endpoint)
  - `pep-gateway` (HTTP proxy/ext_authz-style; includes MCP + CLI wrapper stubs)
- OpenAPI skeleton: `/v1/decision`, `/v1/policies`, `/v1/tools`
- Telemetry stubs: OpenTelemetry tracing + log correlation + request IDs

## Quickstart
```bash
make dev
# or
make up
make seed
```

Services (default local ports):
- Control Plane API: http://localhost:8080
- PDP:              http://localhost:8081
- PEP Gateway:      http://localhost:8082
- UI (Next.js):     http://localhost:3000
- Jaeger:           http://localhost:16686

## Docs
- Executive Summary: `docs/exec_summary.md`
- Vision: `docs/vision.md`
- Status: `docs/status.md`
- Playbook: `docs/playbook.md`
- Next steps: `docs/next_steps.md`

## Demo
Follow `docs/runbooks/demo.md`.

## Docs
- Development workflow: `docs/how-to-develop.md` (references `RULES.md`)
- API spec: `docs/api/openapi.yaml`
- Architecture + threat model: `docs/architecture/*`
- ADRs: `docs/adr/*`

## Deployment intent
**Customer-only today** (on-prem / customer VPC), with a clear path to **hybrid** later.
See `docs/adr/0004-deploy-model-customer-only-migrate-to-hybrid.md`.

---
**Note:** If you have an existing Umbra “umbrella docs” folder (spec/market/phases), keep it adjacent in your Umbra homebase.
This repo focuses on the V0-C product scaffold.


## Process
- Backlog: `docs/process/backlog.md`
- Definition of Done: `docs/process/definition-of-done.md`
- Release train: `docs/process/release-train.md`
- Risk register: `docs/process/risk-register.md`
- Working agreements: `docs/process/working-agreements.md`


## Reference
- Repo map: `docs/repo_map.md`
- Acronyms: `docs/acronyms.md`

## Build verification (recommended)
Run the same checks CI runs (Go + UI):

```bash
make verify
```

Notes:
- First run will download Go/Node dependencies (needs network access).
- If you’re offline or behind a restricted proxy, run from an environment with registry/proxy access.

## Documentation
- Start here: `docs/00_exec_summary.md`
- Demo playbook: `docs/04_playbook_demo.md`
