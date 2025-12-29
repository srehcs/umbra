# Agent Task Prompt (copy/paste)

You are implementing **Ticket: <ID — title>** in the Umbra monorepo.

## Non-negotiables
- Follow `umbra/RULES.md` (type-safe, no emojis, bounded timeouts/retries, no secrets in logs/receipts, structured logs, OTel correlation, tests).
- Keep changes scoped to `umbra/identity/control-plane/` unless the ticket explicitly requires otherwise.
- Do NOT create new doc locations; only update `umbra/identity/control-plane/docs/`.

## Ticket requirements
<PASTE THE GITHUB ISSUE BODY HERE>
0003 — Observe vs Enforce Mode (PEP_MODE)

Goal/Objective: We need to differentiate ourselves by not just being an observability layer watching what actions agents can (and will attempt) to take; guardrail enforcement for them should be built out and we can start doing that by having the ability to push policy.

Delivery Criteria (Rough Outline)

Config
PEP_MODE supports:
observe
enforce
Mode is set via environment variable (docker-compose + documented)
Behavioral guarantees
When decision is DENY:
observe forwards request and returns tool response
receipt records decision=deny and enforcement=forwarded
enforce blocks request
receipt records decision=deny and enforcement=blocked
When decision is ALLOW:
both modes forward request
receipt records decision=allow and enforcement=forwarded
Response contract for blocked requests (enforce)
returns stable error code (example: POLICY_DENIED)
includes request_id
includes decision_id (if generated)
does not echo sensitive tool args
Receipts include mode and outcome
receipt stores:
pep.mode
decision.result
enforcement.outcome (blocked|forwarded)
Tests
Integration tests cover:
deny in observe vs enforce (different outcomes)
allow in both modes (same outcome)
Verification Steps

Set PEP_MODE=observe, run a denied request → forwarded; receipt shows enforcement=forwarded
Set PEP_MODE=enforce, run same request → blocked; receipt shows enforcement=blocked
Confirm receipts include pep.mode and correlation ids

## Where to implement (start here; keep scope tight)
- Read the repo map and contextual files as needed to make a determination.
- We are staying in /identity and in the /workspaces/umbra/identity/control-plane/services


## Output required
- Summary of changes (bullets)
- Files changed (list)
- How to verify (commands + expected results)
- Risks/rollback notes (if behavior changes)
- Log your changes in a 0003 folder inside /tickets in our repo like we did for 0001 and 0002.
