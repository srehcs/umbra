#!/usr/bin/env bash
set -euo pipefail

TENANT_ID="${TENANT_ID:-00000000-0000-0000-0000-000000000001}"

curl -sS -X POST "http://localhost:8081/v1/decision" \
  -H "content-type: application/json" \
  -d '{
    "tenant": {"tenant_id": "'${TENANT_ID}'"},
    "actor": {"type":"user","id":"user-1","roles":["developer"]},
    "tool": {"name":"demo.http","method":"GET","endpoint":"/demo"},
    "trace": {"request_id":"demo-req-1"}
  }' | jq .
