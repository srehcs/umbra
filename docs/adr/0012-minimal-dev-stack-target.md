# ADR-0012: Minimal dev stack target (make dev-min)

- Status: accepted
- Date: 2026-01-02
- Owners: Umbra team

## Context
The full local stack includes enforcement and demo services that are not required for basic UI, policy, and receipt workflows. Contributors working on the control plane UI/API should be able to avoid starting optional services and their dependencies.

## Decision
Add a `make dev-min` target that starts the minimum local stack: UI, controlplane-api, PDP, and their required dependencies. Optional services (PEP, MCP demo services, observability extras) remain available via the full `make dev` or explicit opt-in targets.

Container breakdown:
- Included (dev-min):
  - `postgres` — primary database
  - `controlplane-api` — admin API for tools, policies, receipts
  - `pdp` — policy decision point (allow/deny)
  - `ui` — Next.js control plane console
- Excluded (dev-min):
  - `redis` — caching/eventing dependency (optional in dev)
  - `otel-collector` — OpenTelemetry collector (observability pipeline)
  - `jaeger` — local trace viewer (observability UI)
  - `pep-gateway` — enforcement proxy for tool calls
  - `mcp-adapter` — MCP-facing enforcement adapter
  - `mcp-upstream` — MCP test upstream service
  - `upstream-sample` — demo upstream for PEP forwarding
  - `pep-rust` — Rust PEP gateway (optional profile)

Observability profile:
- `make dev` enables the `obs` profile by default (Redis + OTel Collector + Jaeger).
- `make dev-min` runs without `obs` (no extra containers).

## Alternatives considered
1) Keep a single `make dev` flow for all contributors.
2) Maintain a separate compose file for minimal services.

## Consequences
- Positive: Faster local startup and lower resource usage for common UI/API work.
- Positive: Clear separation between core control-plane workflows and optional demo/enforcement services.
- Negative: Contributors may need to switch targets when they start testing enforcement flows.
- Follow-ups: Keep developer docs and READMEs in sync with the `dev-min` target.
