# Umbra

Umbra is the homebase for the projects we’re considering and actively building.

Umbra is building a **security bouncer for AI agents** inside a company. As more teams let AI assistants (agents) do real work—run scripts, call internal APIs, query databases, deploy code—companies need a way to ensure those actions are **controlled, explainable, and auditable**.

Umbra answers:
- Who (person or agent) attempted an action?
- What tool did they try to use?
- What exactly were they trying to do?
- Should it be allowed or blocked—and why?
- What’s the tamper-evident record (receipt) of what happened?

---

## Current projects

### Identity

**Agent Identity Control Plane (V0-C)**  
An enterprise-facing control plane that evaluates tool invocations (PDP), enforces decisions (PEP), and emits audit receipts (hash-chained, signing-ready).

Repo path: `identity/control-plane/`

Key folders:
- UI (Next.js + ShadCN): `identity/control-plane/ui/`
- Services (Go): `identity/control-plane/services/`
- Shared packages: `identity/control-plane/packages/`
- Docs (ADRs, C4, OpenAPI, threat model): `identity/control-plane/docs/`

What this enables (in plain language):
- A **decision brain** (PDP) that decides allow/deny
- An **enforcement gate** (PEP) that blocks or forwards tool calls
- A **receipt ledger** so you can prove who did what and why it was allowed

### Mesh
Reserved for mesh/observability concepts and comparisons (not the active build).

### Setup
Workflows and development setup references.

---

## Quick demo (local)

From the repo root:

```bash
cd identity/control-plane
make dev
make seed