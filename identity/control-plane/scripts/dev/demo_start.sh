#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

COMPOSE="docker compose -f deployments/docker-compose.yml"
COMPOSE_PROFILES="${COMPOSE_PROFILES:-obs}"
PEP_MODE="${PEP_MODE:-enforce}"

echo "[demo-start] Starting Umbra demo stack (COMPOSE_PROFILES=$COMPOSE_PROFILES, PEP_MODE=$PEP_MODE)"
mkdir -p ./local/.data/postgres
COMPOSE_PROFILES="$COMPOSE_PROFILES" PEP_MODE="$PEP_MODE" $COMPOSE up -d --build

echo "[demo-start] Waiting for service readiness"
bash scripts/dev/wait_for_services.sh

echo "[demo-start] Seeding data"
bash scripts/dev/seed_db.sh

echo "[demo-start] Running demo readiness checks"
bash scripts/dev/demo_check.sh

tenant_id_by_name() {
  local name="$1"
  COMPOSE_PROFILES="$COMPOSE_PROFILES" PEP_MODE="$PEP_MODE" $COMPOSE exec -T postgres \
    psql -U umbra -d umbra -tA -c "SELECT id FROM tenants WHERE name='${name}' ORDER BY created_at DESC LIMIT 1;" | xargs
}

TENANT_A="$(tenant_id_by_name TenantA)"
TENANT_B="$(tenant_id_by_name TenantB)"

if [[ -z "$TENANT_A" || -z "$TENANT_B" ]]; then
  echo "[demo-start] Failed to resolve TenantA/TenantB IDs after seed"
  exit 1
fi

cat <<EOF

[demo-start] Demo stack is ready.

Tenants:
  TenantA=$TENANT_A
  TenantB=$TENANT_B

Endpoints:
  UI:  http://localhost:3000
  API: http://localhost:8080
  PDP: http://localhost:8081
  PEP: http://localhost:8082
  Jaeger: http://localhost:16686

Verification commands:
  curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/tools | jq .
  curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/policies | jq .

  # Allow path (expected: 200 OK + hello-from-upstream)
  curl -i -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8082/tool/demo

  # Deny path (expected: 403 Forbidden when PEP_MODE=enforce)
  curl -i -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8082/tool/secret

  # Receipts list + integrity verify
  curl -s -H "x-umbra-tenant-id: $TENANT_A" http://localhost:8080/v1/receipts | jq .
  curl -s -X POST -H "x-umbra-tenant-id: $TENANT_A" "http://localhost:8080/v1/receipts/verify?kind=all&limit=100" | jq .

EOF
