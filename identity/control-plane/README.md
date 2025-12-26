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

Open:
- UI: http://localhost:3000
- Control Plane API: http://localhost:8080
- PDP: http://localhost:8081
- PEP Gateway: http://localhost:8082

Demo script:
- `docs/04_playbook_demo.md`
- Examples: `docs/examples/`

---

## Documentation

Start here:
- `docs/00_exec_summary.md`
- `docs/04_playbook_demo.md`

Engineering rules (umbrella-level):
- `../../RULES.md`

---

## Technical details (legacy)

We keep the original technical README here:
- `README.legacy.md`
