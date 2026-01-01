# How to Develop

This repository follows Umbra's shared engineering rules:

- **Umbra RULES**: `../../RULES.md`

## Local development
From `identity/control-plane/`:

```bash
make dev
make seed
```

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
