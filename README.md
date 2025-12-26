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

# Umbra - Identity Project

Umbra is building a kind of **security bouncer for AI agents** inside a company.

As more teams let AI assistants (agents) do real work—run scripts, call internal APIs, query databases, deploy code—companies need a way to ensure those actions are **controlled, explainable, and auditable**.

Umbra answers questions like:
- **Who** (person or agent) tried to do something?
- **What tool** did they try to use?
- **What exactly** were they trying to do?
- **Should it be allowed or blocked?**
- And either way: **what’s the tamper-evident record of what happened?**

---

## The project you should look at first

### Agent Identity Control Plane (V0‑C)
Path: `identity/control-plane/`

This is the first product slice:
- A **Policy Decision** service (PDP) that decides allow/deny (+ obligations later)
- A **Policy Enforcement** gateway (PEP) that sits in front of tool calls
- A **Receipt ledger** (audit trail) that’s hash-chained and signing-ready
- A **Web console** (Next.js + ShadCN) to manage tools/policies and inspect receipts

If you only have 5 minutes: start the demo and look at receipts.

---

## Quick demo (local)

From the repo root:

```bash
cd identity/control-plane
make dev
make seed
```

Then open:
- UI: http://localhost:3000
- Receipts and policy actions will show up in the console.

---

## Why this is interesting (enterprise angle)

Companies don’t just need “cool agents”—they need:
- **default-deny** and explicit policy,
- **least privilege** (credential brokering comes next),
- **audit trails** that stand up in incident response,
- and a clean path to **SSO (Keycloak/OIDC)** and compliance posture.

Umbra is built to become the standard layer between agents and tools.

---

## Engineering standards

Umbra has non‑negotiable engineering rules:
- `RULES.md`

---
