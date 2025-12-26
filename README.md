# Umbra

Umbra is the **homebase** for the projects we are considering and actively building.

## Current projects

### Identity
- **Agent Identity Control Plane (V0-C)**: enterprise-facing control plane that evaluates tool invocations (PDP),
  enforces decisions (PEP), and emits audit receipts (hash-chained, signing-ready).

  Repo path: `umbra/identity/control-plane/`

  - UI (Next.js + ShadCN): `umbra/identity/control-plane/ui/`
  - Services (Go): `umbra/identity/control-plane/services/`
  - Packages: `umbra/identity/control-plane/packages/`
  - Docs (ADRs, C4, OpenAPI, threat model): `umbra/identity/control-plane/docs/`

### Mesh
- Reserved for mesh/observability concepts and comparisons (not the active build).

### Setup
- Workflows and development setup references.

## How to work in this monorepo
- Engineering rules: `RULES.md`
- For control-plane dev: see `identity/control-plane/docs/how-to-develop.md`
