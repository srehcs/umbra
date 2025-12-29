# Ticket 0002 — Deterministic Local Demo

## Summary of changes ✅
- Implemented deterministic `make dev` flow that waits for readiness and prints endpoints.
- Hardened `scripts/dev/seed_db.sh` to wait for Postgres and to trim tenant IDs; seed operations are idempotent (use upserts).
- Added `scripts/dev/wait_for_services.sh` to poll health endpoints and Postgres readiness.
- Added `scripts/dev/demo_check.sh` and `make demo-check` target to verify DB/API/PDP/PEP, receipts table, and presence of at least one tool and policy.
- Updated `Makefile` to wire `dev` and `demo-check` targets and ensure local data dir is created.

## Files changed
- `Makefile` (dev, demo-check, seed wiring)
- `scripts/dev/seed_db.sh` (wait for postgres, trimming tenant ids, idempotent upserts)
- `scripts/dev/wait_for_services.sh` (new)
- `scripts/dev/demo_check.sh` (new)
- `docs/0002_deterministic_local_demo.md` (original location — moved here)

## How to verify (recommended steps)
1. Fresh clone the repo and change to control-plane:

   cd identity/control-plane

2. Start the local stack and seed (from a machine with Docker):

   make dev

   - Expected: `docker compose` builds and starts containers, script waits for readiness, seed runs, and the following printed:
     - UI: http://localhost:3000
     - API: http://localhost:8080
     - PDP: http://localhost:8081
     - PEP: http://localhost:8082
     - `make dev` exits 0 when services are ready.

3. Verify the demo state:

   make demo-check

   - Expected: exits 0 and prints `All checks passed`.
   - Verifies: docker-compose config OK, Postgres reachable, receipts table present, API/PDP/PEP healthy, at least one tenant/tool/policy present, receipts queryable.

4. Re-run seed and verify idempotency:

   make seed
   make demo-check

   - Expected: `make seed` exits 0, running it multiple times does not create duplicates and `make demo-check` still passes.

5. Smoke test the UI/API manually:

   - UI should load at http://localhost:3000
   - API health: http://localhost:8080/healthz
   - PDP health: http://localhost:8081/healthz

## Risks / Rollback notes ⚠️
- Changes are limited to the `identity/control-plane/` directory and are non-invasive.
- If the new scripts cause issues for developers on machines without Docker installed, they can still run `make up` and `make seed` manually.
- Rollback: revert the commits touching the files listed above to return to prior behavior.

---

If you'd like, I can also add a small CI job (GitHub Actions) to run `docker compose config` and `scripts/dev/demo_check.sh` on PRs to prevent regressions.