# Umbra

Umbra is building the **security layer for AI agents** in the enterprise: a control plane that sits between agents and the tools they use, deciding what actions are allowed, enforcing those decisions, and recording a tamper-evident audit trail.

As companies adopt agents to ship code, query data, and operate production systems, they need answers that hold up under scrutiny:
- **Who** (human or agent) attempted an action?
- **What tool** did they try to use, and **what exactly** were they trying to do?
- **Was it allowed or blocked—and why?**
- **What’s the receipt** we can trust later during incident response or compliance review?

Umbra is also the homebase for the projects we’re considering and actively building.

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

What this enables (plain language):
- A **decision brain** (PDP) that returns allow/deny (+ obligations later)
- An **enforcement gate** (PEP) that blocks or forwards tool calls
- A **receipt ledger** so you can prove who did what, when, and why it was allowed

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
```

Then open:
- UI: http://localhost:3000

Demo guide:
- `identity/control-plane/docs/04_playbook_demo.md`

---

## Project management

- Next steps backlog (Linear format): `NEXTSTEPS.md`

---

## Engineering standards

Non-negotiable engineering rules:
- `RULES.md`

For control-plane development workflow:
- `identity/control-plane/docs/how-to-develop.md`
