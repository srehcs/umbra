#!/usr/bin/env bash
set -euo pipefail

TENANT_ID="${TENANT_ID:-00000000-0000-0000-0000-000000000001}"

curl -sS -X POST "http://localhost:8082/demo" \
  -H "content-type: application/json" \
  -H "x-umbra-tenant-id: ${TENANT_ID}" \
  -d '{"tool":"demo.http","method":"GET","path":"/demo","actor":{"id":"user-1","roles":["developer"]}}' | jq .
