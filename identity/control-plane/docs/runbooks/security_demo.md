# Security Demo Runbook

Audience: security/platform stakeholders evaluating enforcement, controlled change, and auditability.

Goal: show a denied tool call, a policy change that flips the outcome, and receipts that prove what happened.

## Prerequisites
- Docker Desktop running
- `make` available
- `curl` and `jq` installed

## 0) Start stack + seed
From `identity/control-plane/`:
```bash
make dev
make seed
```

The seed output prints tenant IDs (e.g., TenantA/TenantB). Pick one and export it:
```bash
export TENANT_ID="<tenant-id-from-seed>"
```

Optional (UI auto-select tenant on first load):
```bash
export NEXT_PUBLIC_TENANT_ID="$TENANT_ID"
```

## 1) Verify system health
```bash
curl -s http://localhost:8080/healthz
curl -s http://localhost:8081/healthz
curl -s http://localhost:8082/healthz
```

Expected:
- Each returns `ok` (HTTP 200).

## 2) Baseline policy view (prove current state)
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8080/v1/policies | jq .
```

Expected:
- HTTP 200.
- At least one policy visible (seeded policy is usually active).

## 3) Demonstrate prevention (deny)
Use the PEP to invoke an MCP-style tool call that is not allowed by policy.
```bash
curl -i -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8082/tool/secret
```

Expected:
- HTTP `403 Forbidden` (enforce mode).
- Response body includes a stable error message for denied enforcement.
Note:
- If `PEP_MODE=observe`, the request is forwarded and returns `200 OK`, but the receipt should indicate a deny decision.

UI check:
- Open http://localhost:3000 and navigate to Receipts.
- You should see a new receipt for this blocked invocation.
- Open the receipt detail and confirm `request_id` and `decision_id` are present.

## 4) Controlled change (policy update)
Flip policy outcome to allow the same tool call.

Option A (UI):
1) Navigate to Policies.
2) Edit the active policy to allow `GET /secret` for the current role.
3) Activate the updated policy.

Option B (API, if you prefer scripted):
- Create or update a policy to allow the path, then activate it via the policies endpoint.
- Use existing policy tooling if available in the UI.

Expected:
- UI shows the new policy as active.
- Active policy hash changes (note it for later).

## 5) Demonstrate allow (same call now succeeds)
```bash
curl -i -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8082/tool/secret
```

Expected:
- HTTP `200 OK`.
- Body from upstream service (if configured).

UI check:
- Receipts show a new invocation receipt with `decision=allow`.
- In receipt detail, confirm `policy_hash` matches the newly activated policy.

## 6) Proof: receipts integrity verification
```bash
curl -s -X POST -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts/verify?kind=all&limit=100" | jq .
```

Expected:
- HTTP 200.
- `status` (or equivalent) indicates PASS.
- If it fails, the response includes the first failing receipt id/hash.
Note:
- In dev, `HASH_MISMATCH` usually means you are verifying receipts created by an older build or a previous DB state. If needed, reset the dev Postgres volume and reseed, then rerun the verify step. See `docs/receipts_integrity.md`.

## 7) Proof: export receipts (JSON and CSV)
Export receipts for audit review.
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts/export?format=json&limit=200" | jq .
```

Expected:
- HTTP 200.
- JSON array or JSONL with receipts including `request_id`, `decision_id`, and `policy_hash`.

CSV export:
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts/export?format=csv&limit=200"
```

Expected:
- HTTP 200.
- CSV headers with receipt fields.

## 8) Close the loop (what the audience should hear)
- Enforcement happens **before** the tool call executes (deny prevents action).
- Policy changes are controlled and auditable (active policy hash changes).
- Receipts provide a tamper-evident trail with correlation IDs and policy hash.
- Export and verification enable incident response and compliance review.

## Troubleshooting
- If you get `storage not configured`, restart services after Postgres is ready:
  ```bash
  docker compose -f deployments/docker-compose.yml restart controlplane-api pdp
  ```
- If UI requests fail, verify `x-umbra-tenant-id` is set (tenant switcher) and the API proxy is configured.
