#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"
TENANT_ID="${E2E_TENANT_ID:-11111111-1111-1111-1111-111111111111}"

echo "[e2e] Starting stack"
if [ "${E2E_SKIP_UP:-0}" = "1" ]; then
  echo "[e2e] Skipping docker compose up"
elif [ "${E2E_NO_BUILD:-0}" = "1" ]; then
  $COMPOSE up -d
else
  $COMPOSE up -d --build
fi

bash scripts/dev/wait_for_services.sh

./scripts/dev/e2e_seed.sh

echo "[e2e] Verifying storage"
if curl -s -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8080/v1/policies | grep -q "storage not configured"; then
  echo "[e2e] Restarting controlplane-api and pdp"
  $COMPOSE restart controlplane-api pdp
fi

echo "[e2e] Generating receipts"
curl -s -H "x-umbra-tenant-id: $TENANT_ID" http://localhost:8082/tool/demo >/dev/null || true

echo "[e2e] Ready. Tenant=$TENANT_ID"
