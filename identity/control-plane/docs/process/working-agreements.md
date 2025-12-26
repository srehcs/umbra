# Working Agreements

These agreements keep velocity high without violating `RULES.md`.

## PR size & tempo
- Aim for ≤ ~400 LOC net per PR (exceptions: generated code/migrations)
- Branches should live < 3 days where possible
- Prefer “vertical slices” over horizontal scaffolding

## Review expectations
- Be explicit about risk and rollback in PR description
- Call out threat model impact when touching:
  - auth, policy semantics, tenant isolation, receipts, migrations

## Interfaces & boundaries
- Keep boundaries clear:
  - control plane API vs PDP vs PEP
- Shared code belongs in `/packages` with an owner; no “utils dumping ground”

## Observability by default
- Any new endpoint must have:
  - span + trace ID propagation
  - structured logs
  - key attrs: tenant_id, decision_id/tool_id (as applicable)

## Security defaults
- Default-deny for policy
- Fail closed when PDP unreachable (unless explicitly ADR’d)
- Never log secrets
