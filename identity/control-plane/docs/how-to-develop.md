# How to Develop

This repository follows Umbra's shared engineering rules:

- **Umbra RULES**: `../../RULES.md`

## Local development
From `identity/control-plane/`:

```bash
make dev
make seed
```

Minimal stack (UI + API + PDP only):
```bash
make dev-min
```

Default stack includes the observability profile:
```bash
make dev
```
This enables Redis + OTel Collector + Jaeger via `COMPOSE_PROFILES=obs`.

Optional: auto-seed the UI tenant
Set `NEXT_PUBLIC_TENANT_ID` before starting the UI so the console auto-selects a tenant:
```bash
export NEXT_PUBLIC_TENANT_ID="<tenant-id-from-seed>"
```

Optional: dev auth roles for UI gating
Set roles and user for local testing (client-side only):
```bash
export NEXT_PUBLIC_DEV_ROLES="policy_admin,tool_admin,auditor"
export NEXT_PUBLIC_DEV_USER="dev-user"
```
You can also override roles in the browser with:
```js
localStorage.setItem("umbra.roles", "policy_admin,tool_admin,auditor");
```

Optional: auth session headers (when AUTH_ENABLED=true)
If you front the UI with an auth proxy, forward:
- `x-umbra-user`
- `x-umbra-roles` (comma-separated)
- `x-umbra-tenant-id`

Enable auth session mode (UI reads `/api/auth/session`):
```bash
export AUTH_ENABLED=true
export NEXT_PUBLIC_AUTH_ENABLED=true
```

Example: header-based session fetch
```bash
curl -s \
  -H "x-umbra-user: alice" \
  -H "x-umbra-roles: policy_admin,tool_admin,auditor" \
  -H "x-umbra-tenant-id: <tenant-id>" \
  http://localhost:3000/api/auth/session | jq .
```

Receipt idempotency config
```bash
export UMBRA_REQUEST_ID_DEDUPE_WINDOW="24h"
export UMBRA_RECEIPT_CHAIN_LOCK_SCOPE="tenant" # or "day"
```

E2E smoke tests
```bash
make e2e
```
Prereqs:
- Node 20.x LTS (20.11+ recommended) for Playwright stability.
- Run `pnpm -C ui exec playwright install` once to download browsers.

Faster options:
```bash
make e2e-fast  # reuse existing images (skip docker --build)
make e2e-local # assumes stack already running, skips docker compose up
```
Defaults:
- `E2E_TENANT_ID=11111111-1111-1111-1111-111111111111`
- `E2E_ROLES=policy_admin,tool_admin,auditor`

Schema checks
```bash
./scripts/dev/verify_signature_schema.sh
```

Integration test DB guard
```bash
export UMBRA_TEST_DATABASE_URL="postgres://.../umbra_test"
```
Integration tests enforce a dedicated test DB (or per-test schema). Override locally with:
```bash
export UMBRA_ALLOW_NON_TEST_DB=1
```

Fuzz and property tests (local)
```bash
go test ./packages/go/policy -run TestDoesNotExist -fuzz=FuzzEvaluateABACV0Deterministic -fuzztime=10s
go test ./packages/go/receipts -run TestDoesNotExist -fuzz=FuzzCanonicalJSONBytesDeterministic -fuzztime=10s
go test ./packages/go/receipts -run TestDoesNotExist -fuzz=FuzzCanonicalizeIdempotencyPayloadDeterministic -fuzztime=10s
```
Seed corpus files live under `packages/go/**/testdata/fuzz/` and are intended to keep fuzzing reproducible.

Stopping the stack
```bash
docker compose -f deployments/docker-compose.yml down
```
Use `down -v` if you also want to wipe volumes and seeded data.

## Services
- controlplane-api: http://localhost:8080
- pdp: http://localhost:8081
- pep-gateway: http://localhost:8082
- ui: http://localhost:3000

## Code organization
- `services/`: runnable processes (controlplane-api, pdp, pep-gateway)
- `packages/`: shared libraries (policy evaluation, receipts, storage)
- `docs/`: OpenAPI, architecture, playbooks (ADRs in `/docs/adr/`)

## Standards
- No secrets in logs
- Deterministic serialization for receipts
- Strict request validation (bounded inputs)
- Trace/Request ID propagation end-to-end
