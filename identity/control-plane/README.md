# Agent Identity Control Plane (V0‑C)

This project is the **front door** for AI agent tool usage in an enterprise.

It sits between agents and the tools they try to use, so the company can reliably answer:

- **Who** tried to do something (human or agent)?
- **What tool** did they try to use?
- **What action** were they attempting?
- **Was it allowed or blocked—and why?**
- **What’s the tamper‑evident record** (receipt) of what happened?

Think: **badge reader + security guard + camera log**, but for software actions.

---

## How it works (three pieces)

1) **Control Panel (UI + Control Plane API)**  
Admins register tools, write policies, and search receipts.

2) **Decision Brain (PDP)**  
Evaluates policies and returns **allow/deny** (plus obligations later).

3) **Enforcement Gate (PEP Gateway)**  
Intercepts tool calls, asks PDP, blocks or forwards, and always writes a receipt.

---

## Quick start (local)

```bash
make dev
make seed
```

For a minimal stack (UI + API + PDP only):

```bash
make dev-min
```

`make dev` enables the `obs` profile by default (Redis + OTel Collector + Jaeger).

Open:
- UI: http://localhost:3000
- Control Plane API: http://localhost:8080
- PDP: http://localhost:8081
- PEP Gateway: http://localhost:8082

Demo script:
- `docs/runbooks/demo.md`
- `docs/runbooks/demo_walkthrough.md`
- `docs/runbooks/mtls_deploy_note.md`
- `docs/runbooks/trace_demo.md`
- `docs/runbooks/security_demo.md`
- PDP unavailable runbook: `docs/runbooks/pdp_unavailable.md`
- Examples: `docs/examples/`

---

## Documentation

Start here:
- `docs/README.md` (index)
- `docs/00_exec_summary.md`
- `docs/runbooks/demo.md`
- `docs/runbooks/demo_walkthrough.md`
- `docs/security/mtls.md`
- `docs/runbooks/trace_demo.md`
- `docs/runbooks/security_demo.md`
- `docs/runbooks/pdp_unavailable.md`

Engineering rules (umbrella-level):
- `../../RULES.md`

---

## API notes

Receipts export endpoint:
- `GET /v1/receipts/export` (format=json|csv)

See `docs/api/openapi.yaml` for the full contract.

---

## CI expectations

- Go: `go test ./...` (run from `identity/control-plane`)
- UI: `pnpm lint` (run from `identity/control-plane/ui`)

---
