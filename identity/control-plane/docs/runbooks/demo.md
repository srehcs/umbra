# Demo Runbook (V0-C)

This runbook is the **definition of demo-ready**.

## 0) Start stack
```bash
make dev
```

The seed script prints two tenant IDs:
- TenantA
- TenantB

Copy one of them for the `x-umbra-tenant-id` header.

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
- `403 Forbidden`

## 4) View receipts
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/receipts | jq .
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
