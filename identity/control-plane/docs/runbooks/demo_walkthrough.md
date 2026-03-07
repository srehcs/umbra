# Evaluation Walkthrough (Demo)

Audience: evaluators and stakeholders who want a fast proof of policy enforcement, traceability, and receipt integrity.

## Prerequisites
- Docker + Docker Compose running
- `curl` and `jq` installed
- From `identity/control-plane/`, run:

```bash
make demo
```

Set a tenant from demo output:
```bash
export TENANT_ID="<TenantA-from-make-demo>"
```

## Step 1) Confirm seeded policy/tool baseline
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8080/v1/tools | jq .
curl -s -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8080/v1/policies | jq .
```

Expected (representative):
```json
{
  "count": 1,
  "first": { "name": "sample-http-tool", "kind": "http" }
}
```
```json
{
  "count": 1,
  "first": {
    "name": "default-policy",
    "active": true,
    "version": 1,
    "policy_hash": "<sha256>"
  }
}
```

## Step 2) Allowed request through PEP
```bash
curl -i -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8082/tool/demo
```

Expected:
- HTTP `200 OK`
- Response body contains `hello-from-upstream`

## Step 3) Denied request through PEP
```bash
curl -i -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8082/tool/secret
```

Expected:
- HTTP `403 Forbidden` in enforce mode
- Error envelope includes `error.code=POLICY_DENIED`, plus `request_id`, `decision_id`, `trace_id`

Example response body:
```json
{
  "error": {
    "code": "POLICY_DENIED",
    "message": "default deny (no matching rule)"
  },
  "request_id": "<uuid>",
  "decision_id": "<uuid>",
  "trace_id": "<trace-id>"
}
```

## Step 4) Receipts correlation proof
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" "http://localhost:8080/v1/receipts?limit=3" | jq .
```

Expected:
- `items[0].kind` is usually `invocation`
- `request_id`, `decision_id`, and `trace_id` are present
- `decision`/`outcome` reflects the call (`allow` or `denied`)

Representative snippet:
```json
{
  "kind": "invocation",
  "request_id": "<uuid>",
  "decision_id": "<uuid>",
  "trace_id": "<trace-id>",
  "outcome": "denied"
}
```

## Step 5) Verify receipt integrity chain
```bash
curl -s -X POST -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts/verify?kind=all&limit=100" | jq .
```

Expected:
```json
{
  "checked": "<n>",
  "kind": "all",
  "ok": true
}
```

## Step 6) UI confirmation (optional but recommended)
- Open `http://localhost:3000/receipts`
- Locate the deny event and open details
- Confirm `request_id`, `decision_id`, `trace_id`, `policy_hash`

Screenshot placeholders:
- `[screenshot-placeholder: receipts-list-deny-event]`
- `[screenshot-placeholder: receipt-detail-correlation-fields]`

## API endpoints used in this walkthrough
- `GET /v1/tools`
- `GET /v1/policies`
- `GET /v1/receipts`
- `POST /v1/receipts/verify`
- `GET /tool/demo` (PEP)
- `GET /tool/secret` (PEP)
