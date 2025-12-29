# UMBRA-0002 Implementation Summary — Deterministic Local Demo

## Overview
Implemented deterministic local demo flow so `make dev` and `make seed` are reliable, idempotent, and provide a predictable demo state for local development.

## Summary of changes ✅
- Deterministic `make dev` flow that waits for readiness and prints endpoints.
- Hardened `scripts/dev/seed_db.sh` to wait for Postgres and to trim tenant IDs; seed operations are idempotent (upserts).
- Added `scripts/dev/wait_for_services.sh` to poll health endpoints and Postgres readiness.
- Added `scripts/dev/demo_check.sh` and a `make demo-check` target to verify DB/API/PDP/PEP, receipts table, and presence of at least one tool and policy.
- Updated `Makefile` to wire `dev` and `demo-check` targets and ensure local data dir is created.

## Files changed
- `Makefile` (added `dev` and `demo-check` targets, ensured local data dir)
- `scripts/dev/seed_db.sh` (wait for postgres, trim tenant ids, idempotent upserts)
- `scripts/dev/wait_for_services.sh` (new)
- `scripts/dev/demo_check.sh` (new)

## How to verify (quick)
1. Fresh clone and start demo:
   cd identity/control-plane
   make dev
   - Expected: containers build/start, `make dev` waits for readiness, seed runs, printed endpoints:
     - UI: http://localhost:3000
     - API: http://localhost:8080
     - PDP: http://localhost:8081
     - PEP: http://localhost:8082
2. Run demo checks:
   make demo-check
   - Expected: exit 0 and `All checks passed`
3. Re-run idempotency checks:
   make seed
   make demo-check
   - Expected: `make seed` exits 0 repeatedly with no duplicates; demo-check still passes

## Verification
- Manual verification via Makefile targets (`make dev`, `make seed`, `make demo-check`)

## Risks / Rollback notes ⚠️
- Changes are scoped to `identity/control-plane/` only.
- If issues arise, revert the commits touching the listed files.

---

If you'd like, I can also add a lightweight CI job to run `docker compose config` and `scripts/dev/demo_check.sh` on PRs to prevent regressions.