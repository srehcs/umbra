# How to Develop

This repository follows Umbra's shared engineering rules:

- **Umbra RULES**: `../../RULES.md`

## Local development
From `identity/control-plane/`:

```bash
make demo
make dev
make seed
```

`make demo` is the one-command demo bring-up. It starts services, waits for readiness,
seeds data, runs demo checks, and prints verification curl commands with tenant IDs.

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

Optional: auth mode (UI + API)
Use these together when you want the control-plane API and UI session route to require a verified bearer token:
```bash
export UMBRA_AUTH_ENABLED=true
export NEXT_PUBLIC_AUTH_ENABLED=true
export UMBRA_AUTH_JWT_HS256_SECRET="<dev-only-secret>"
```
Provider-backed OIDC mode:
```bash
export OIDC_ISSUER_URL="https://issuer.example/realms/umbra"
export OIDC_CLIENT_ID="umbra-ui"
export AUTH_BASE_URL="http://localhost:3000"
```
Optional:
```bash
export OIDC_CLIENT_SECRET="<provider-client-secret-if-needed>"
export OIDC_JWKS_URL="https://issuer.example/realms/umbra/protocol/openid-connect/certs"
```
Compatibility note: the UI server also accepts `AUTH_ENABLED=true`, but `UMBRA_AUTH_ENABLED=true` is the canonical server-side flag.

Optional local-only fallback for the current in-progress auth ticket:
```bash
export NEXT_PUBLIC_AUTH_DEV_TOKEN_ENABLED=true
```
Required claim validation when auth is enabled:
```bash
export UMBRA_AUTH_JWT_ISSUER="https://issuer.example"
export UMBRA_AUTH_JWT_AUDIENCE="umbra-controlplane"
```

Production ingress hardening:
- mTLS edge pattern and certificate/header mapping: `docs/security/mtls.md`
- Validation checklist: `docs/runbooks/mtls_deploy_note.md`

In auth mode, the UI no longer trusts browser-supplied `x-umbra-user`, `x-umbra-roles`, or `x-umbra-tenant-id` headers. It derives session state from a verified JWT and forwards only `Authorization: Bearer ...` to the control-plane proxy.

Provider-backed browser flow:
- Sign in at `http://localhost:3000/api/auth/login`
- The callback exchanges the OIDC code for an access token and stores it in an HTTP-only cookie
- `GET /api/auth/session` and `/api/controlplane/*` then use the server-held session token

Example: session fetch with bearer token
```bash
curl -s \
  -H "Authorization: Bearer <dev-jwt>" \
  http://localhost:3000/api/auth/session | jq .
```

For local UI testing, you can enable the dev-token fallback above and paste the token into the sidebar "Dev token" control. The UI will exchange it for an HTTP-only cookie-backed session.

This fallback is intended for local development only. In production-oriented builds, disable it and use the provider-backed auth flow.

The JWT must carry:
- `sub` or `preferred_username`
- `tenant_id` (UUID)
- one or more roles via `roles`, `realm_access.roles`, `resource_access.umbra.roles`, or `groups`
- if you use `groups`, use exact role names or `/umbra/<role>` group paths (for example `/umbra/policy_admin`)

Receipt idempotency config
```bash
export UMBRA_REQUEST_ID_DEDUPE_WINDOW="24h"
export UMBRA_RECEIPT_CHAIN_LOCK_SCOPE="tenant" # or "day"
```

Optional: local receipt signing (placeholder key)
```bash
export UMBRA_RECEIPT_SIGNING_ENABLED=true
export UMBRA_RECEIPT_SIGNING_REQUIRED=false # set true to fail closed
export UMBRA_RECEIPT_SIGNING_KID="key://local-dev"
export UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM="<pem-with-escaped-newlines>"
```

Note: `POST /v1/receipts` rejects client-supplied `signature_*`/`signed_at` fields.
Signature metadata is generated server-side when signing is enabled.

Trace smoke check
```bash
TENANT_ID="<tenant-id-from-seed>" bash scripts/dev/trace_smoke.sh
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
