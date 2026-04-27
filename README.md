<h1 align="center">Umbra — The Security Layer for Agents</h1>

<p align="center">
  <img src="docs/assets/umbra-header.png" alt="Umbra header" width="900" />
</p>

---

## Overview

Umbra is building the **security layer for AI agents** in the enterprise: a control plane that sits between agents and the tools they use, deciding what actions are allowed, enforcing those decisions, and recording a tamper-evident audit trail.

As companies adopt agents to ship code, query data, and operate production systems, they need answers that hold up under scrutiny:
- **Who** (human or agent) attempted an action?
- **What tool** did they try to use, and **what exactly** were they trying to do?
- **Was it allowed or blocked--and why?**
- **What's the receipt** we can trust later during incident response or compliance review?

Umbra is the homebase for the active build below plus shared standards and documentation.

---

## Active build

### Agent Identity Control Plane (V0-C)
An enterprise-facing control plane that evaluates tool invocations (PDP), enforces decisions (PEP), and emits audit receipts (hash-chained, signing-ready).

Repo path: `identity/control-plane/`

Core components:
- UI (Next.js + ShadCN): `identity/control-plane/ui/`
- Services (Go): `identity/control-plane/services/`
- Shared packages: `identity/control-plane/packages/`
- Docs (C4, OpenAPI, threat model): `identity/control-plane/docs/`
- ADRs (centralized): `docs/adr/`

What this enables (plain language):
- A **decision brain** (PDP) that returns allow/deny (+ obligations later)
- An **enforcement gate** (PEP) that blocks or forwards tool calls
- A **receipt ledger** so you can prove who did what, when, and why it was allowed

<p align="center">
  <img src="docs/assets/umbra-ui.png" alt="Umbra UI" width="840" />
</p>

---

## Quick demo (local)

For local demo steps, see:
- `identity/control-plane/docs/runbooks/demo.md`
- `identity/control-plane/docs/runbooks/pdp_unavailable.md`

---

## Navigation

- Repo map: `REPO_MAP.md`
- Engineering rules: `RULES.md`
- Security policy: `SECURITY.md`
- Contributing guide: `CONTRIBUTING.md`
- License: `LICENSE`
- Control-plane development workflow: `identity/control-plane/docs/how-to-develop.md`
- Control-plane docs index: `identity/control-plane/docs/README.md`

## Project status

Umbra is in active development and early evaluation. APIs and docs may change.

Current implementation state:
- Demo bring-up, evaluator walkthrough, trace correlation, mTLS deployment guidance, and receipt signing are implemented locally in `identity/control-plane/`.
- Control-plane auth is now provider-capable locally: the UI supports an OIDC login/callback flow with HTTP-only cookie sessions, and controlplane API role/tenant checks are claim-derived when auth is enabled.
- Database-level multi-tenant isolation, KMS-backed signing, and broader production hardening remain in the next work queue.

## Contributing

We follow the engineering standards in `RULES.md`. If you plan to contribute, read the rules first.

## Assets

Store README images and GitHub icon assets in `docs/assets/` and reference them with relative paths.

---

## Inactive or reserved

- `archive/` -- historical material
