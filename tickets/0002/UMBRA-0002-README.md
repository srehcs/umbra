# UMBRA-0002 Quick Reference — Deterministic Local Demo

## What this ticket delivers
- Deterministic `make dev` flow (waits for readiness, seeds DB)
- Idempotent `make seed` (upserts, stable tenant IDs)
- Demo verification via `make demo-check` (DB, services, receipts, tool/policy presence)

## Short verification checklist
- make dev → exits 0 and prints endpoints
- make demo-check → exits 0 and prints `All checks passed`
- make seed (repeat) → idempotent

## Useful commands
- Start & seed: `make dev`
- Check demo health: `make demo-check`
- Re-seed & verify: `make seed && make demo-check`

## Scripts in this ticket
- None — use Makefile targets directly (`make dev`, `make seed`, `make demo-check`)

## Notes
- These changes are scoped to `identity/control-plane/` and are safe to revert by removing the modified files if needed.