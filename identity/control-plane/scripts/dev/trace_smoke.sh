#!/usr/bin/env bash
set -euo pipefail

TENANT_ID="${TENANT_ID:-}"
API_URL="${API_URL:-http://localhost:8080}"
PEP_URL="${PEP_URL:-http://localhost:8082}"
JAEGER_URL="${JAEGER_URL:-http://localhost:16686}"
TRACE_KIND="${TRACE_KIND:-deny}" # allow | deny

if [[ -z "$TENANT_ID" ]]; then
  echo "TENANT_ID is required"
  echo "Example: TENANT_ID=<tenant-uuid> bash scripts/dev/trace_smoke.sh"
  exit 1
fi

if [[ "$TRACE_KIND" != "allow" && "$TRACE_KIND" != "deny" ]]; then
  echo "TRACE_KIND must be allow or deny"
  exit 1
fi

REQUEST_ID="trace-smoke-$(date +%s)"
TARGET_PATH="/tool/demo"
if [[ "$TRACE_KIND" == "deny" ]]; then
  TARGET_PATH="/tool/secret"
fi

echo "[trace-smoke] Sending $TRACE_KIND request with request_id=$REQUEST_ID"
status="$(curl -sS -o /tmp/trace_smoke_resp_body.json -w "%{http_code}" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -H "x-umbra-request-id: $REQUEST_ID" \
  "$PEP_URL$TARGET_PATH")"

echo "[trace-smoke] HTTP status: $status"

sleep 1

receipts_json="$(curl -sS -H "x-umbra-tenant-id: $TENANT_ID" "$API_URL/v1/receipts?q=$REQUEST_ID&limit=5")"
trace_id="$(echo "$receipts_json" | jq -r '.items[0].trace_id // empty')"
span_id="$(echo "$receipts_json" | jq -r '.items[0].span_id // empty')"
decision_id="$(echo "$receipts_json" | jq -r '.items[0].decision_id // empty')"
kind="$(echo "$receipts_json" | jq -r '.items[0].kind // empty')"

if [[ -z "$trace_id" ]]; then
  echo "[trace-smoke] Failed to locate trace_id from receipts for request_id=$REQUEST_ID"
  echo "$receipts_json" | jq .
  exit 1
fi

echo
echo "[trace-smoke] Correlation fields:"
echo "request_id=$REQUEST_ID"
echo "trace_id=$trace_id"
echo "span_id=$span_id"
echo "decision_id=$decision_id"
echo "receipt_kind=$kind"
echo
echo "[trace-smoke] Jaeger:"
echo "Open: $JAEGER_URL"
echo "Search service: pep-gateway.http"
echo "Filter by trace_id: $trace_id"

