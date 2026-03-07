# Demo Runbook (V0-C)

This runbook is the **definition of demo-ready**.

For a stakeholder-facing concise flow with expected outputs, see:
- `docs/runbooks/demo_walkthrough.md`
- `docs/runbooks/trace_demo.md`

## 0) Start stack
Recommended (one command):
```bash
make demo
```

`make demo` starts the stack, waits for readiness, seeds data, runs `demo-check`,
and prints tenant IDs plus verification curl commands.

The output includes two tenant IDs:
- TenantA
- TenantB

Copy one of them for the `x-umbra-tenant-id` header.

Manual equivalent:
```bash
make dev
make seed
make demo-check
```

Optional: auto-seed the UI tenant
Set `NEXT_PUBLIC_TENANT_ID` before starting the UI so the console auto-selects a tenant:
```bash
export NEXT_PUBLIC_TENANT_ID="<tenant-id-from-seed>"
```
If you run the UI via Docker Compose, set `NEXT_PUBLIC_TENANT_ID` in an override file or your shell before `make dev`.

Troubleshooting:
- If any API call returns `storage not configured`, restart the services after Postgres is ready:
  ```bash
  docker compose -f deployments/docker-compose.yml restart controlplane-api pdp
  ```

## 1) Verify health
```bash
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:8082/healthz
```

## 2) List tools/policies for TenantA
```bash
TENANT_A="<paste from seed output>"
curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/tools | jq .
curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/policies | jq .
```

## 3) Enforce a tool invocation through PEP
The seeded policy allows `GET /demo` for roles `admin` or `developer`. PEP sends role `developer` in V0.

```bash
curl -i \
  -H "x-umbra-tenant-id: $TENANT_A" \
  http://localhost:8082/tool/demo
```

Expected:
- `200 OK`
- body from upstream sample: `hello-from-upstream`

Try a denied path:
```bash
curl -i -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8082/tool/secret
```
Expected:
- `403 Forbidden` when `PEP_MODE=enforce` (default for `make demo`)
- `200 OK` when `PEP_MODE=observe`, with receipts still showing a deny decision

## 4) View receipts
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/receipts | jq .
```

## 4a) Verify receipt integrity
```bash
curl -s -X POST -H "x-umbra-tenant-id: $TENANT_A" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100" | jq .
```

## 5) View traces
Open Jaeger:
- http://localhost:16686

Search for service:
- `pep-gateway.http`
- `pdp.http`

You should see spans for decision and proxy enforcement.

## 6) UI
- http://localhost:3000
Set tenant in UI by setting `NEXT_PUBLIC_TENANT_ID` in compose, or call APIs via curl for now.

## UI extras (demo)
- Policies → **Simulate**: test (method,path,roles) vs active policy or draft JSON before activating.
- Receipts → server-side **q/kind** filtering + **Load more** pagination.
