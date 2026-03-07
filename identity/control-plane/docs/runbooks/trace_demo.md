# Trace + Receipt Correlation Demo

Goal: prove end-to-end trace continuity by correlating `request_id`, `trace_id`, and `span_id` between PEP/PDP activity and stored receipts.

## Prerequisites
- Stack running via `make demo`
- A seeded tenant ID
- `jq` installed

```bash
export TENANT_ID="<tenant-id-from-make-demo>"
```

## Option A) Quick smoke script (recommended)
From `identity/control-plane/`:

```bash
TENANT_ID="$TENANT_ID" bash scripts/dev/trace_smoke.sh
```

For an allow trace instead of deny:
```bash
TENANT_ID="$TENANT_ID" TRACE_KIND=allow bash scripts/dev/trace_smoke.sh
```

Expected output fields:
- `request_id=...`
- `trace_id=...`
- `span_id=...`
- `decision_id=...` (when available)

## Option B) Manual trace flow
1) Send one request with an explicit request id:
```bash
export REQUEST_ID="trace-manual-$(date +%s)"
curl -i \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -H "x-umbra-request-id: $REQUEST_ID" \
  http://localhost:8082/tool/secret
```

2) Query receipts by request id:
```bash
curl -s -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts?q=$REQUEST_ID&limit=5" | jq .
```

3) Confirm first receipt includes:
- `request_id` = your request id
- `trace_id` non-empty
- `span_id` non-empty
- `decision_id` present for policy decisions

Representative receipt excerpt:
```json
{
  "kind": "invocation",
  "request_id": "trace-manual-1234567890",
  "decision_id": "0359ff07-85d8-4db0-aae8-3423e3bcae22",
  "trace_id": "baa207d62a14f6be16dcb2791e502dfe",
  "span_id": "f8ea4ad95a0fe6b5"
}
```

## Jaeger query
Open Jaeger:
- `http://localhost:16686`

Suggested query:
- Service: `pep-gateway.http`
- Then filter or inspect traces matching the captured `trace_id`

Optional cross-check:
- Search `pdp.http` for the same trace to validate propagation across PEP -> PDP.

## Success criteria
- The same `request_id` appears in responses/receipts.
- A non-empty `trace_id` and `span_id` is present in receipts.
- Jaeger contains a trace that matches the receipt `trace_id`.
