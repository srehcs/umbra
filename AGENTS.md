# Umbra Agent Guide

## Scope

- Active implementation: `identity/control-plane/`
- Authoritative engineering standards: `RULES.md`
- Security reporting and public-doc constraints: `SECURITY.md`
- Canonical development workflow: `identity/control-plane/docs/how-to-develop.md`

## Common commands

From `identity/control-plane/`:

```bash
make bootstrap
make fmt
make lint
make test
make gen
make dev
make demo
make seed
```

If local Postgres is already using port `5432`, stop it or remap the Docker Compose port before running `make dev`.

## System shape

- `controlplane-api`: admin API for tools, policies, receipts
- `pdp`: policy decision point
- `pep-gateway`: enforcement gateway for tool invocations
- `ui`: Next.js control plane UI

## Public repo note

Keep public-facing instructions minimal at the repo root. Historical planning artifacts live under `docs/archive/planning/`.
