#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"

wait_for_postgres() {
  echo "[seed] Waiting for postgres to be ready..."
  for i in {1..60}; do
    if $COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT 1" >/dev/null 2>&1; then
      echo "[seed] Postgres ready"
      return 0
    fi
    sleep 1
  done
  echo "[seed] Timed out waiting for postgres"
  return 1
}

wait_for_postgres

echo "[seed] Applying migrations..."
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0001_init.sql
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0002_add_request_id.sql
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0003_add_receipt_indexes.sql
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0004_add_receipt_search_indexes.sql
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0005_add_receipt_search_text.sql

echo "[seed] Seeding tenants..."
TENANT_A=$($COMPOSE exec -T postgres psql -U umbra -d umbra -t -c "INSERT INTO tenants(name) VALUES('TenantA') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id;" 2>&1 | grep -oE '[0-9a-f-]{36}' | head -1)
TENANT_B=$($COMPOSE exec -T postgres psql -U umbra -d umbra -t -c "INSERT INTO tenants(name) VALUES('TenantB') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id;" 2>&1 | grep -oE '[0-9a-f-]{36}' | head -1)

if [[ -z "$TENANT_A" || -z "$TENANT_B" ]]; then
  echo "[seed] Failed to compute tenant ids"
  exit 1
fi

echo "[seed] TenantA=$TENANT_A"
echo "[seed] TenantB=$TENANT_B"

echo "[seed] Seeding tools..."
cat <<PSQL | $COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1
  INSERT INTO tools(tenant_id, name, kind, config_json)
  VALUES
    ('$TENANT_A', 'sample-http-tool', 'http', '{"upstream":"http://upstream-sample:9000"}'::jsonb),
    ('$TENANT_B', 'sample-http-tool', 'http', '{"upstream":"http://upstream-sample:9000"}'::jsonb)
  ON CONFLICT(tenant_id, name) DO UPDATE SET config_json=EXCLUDED.config_json, updated_at=now();
PSQL

echo "[seed] Seeding policies..."
POLICY_JSON='{"version":1,"mode":"abac_v0","rules":[{"effect":"deny","mcp_servers_any":["demo.mcp"],"mcp_tools_any":["demo.secret"],"mcp_methods_any":["tools/call"]},{"effect":"allow","mcp_servers_any":["demo.mcp"],"mcp_tools_any":["demo.safe"],"mcp_methods_any":["tools/call"]},{"effect":"allow","roles_any":["admin","developer"],"methods_any":["GET"],"path_prefix":"/demo"}],"default":"deny"}'

$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -c "
  WITH p AS (
    SELECT '$POLICY_JSON'::jsonb AS js, encode(digest('$POLICY_JSON','sha256'),'hex') AS h
  )
  INSERT INTO policies(tenant_id, name, version, active, policy_json, policy_hash)
  SELECT '$TENANT_A', 'default-policy', 1, true, p.js, p.h FROM p
  ON CONFLICT(tenant_id, name, version) DO UPDATE SET active=true, policy_json=EXCLUDED.policy_json, policy_hash=EXCLUDED.policy_hash, updated_at=now();
"

$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -c "
  WITH p AS (
    SELECT '$POLICY_JSON'::jsonb AS js, encode(digest('$POLICY_JSON','sha256'),'hex') AS h
  )
  INSERT INTO policies(tenant_id, name, version, active, policy_json, policy_hash)
  SELECT '$TENANT_B', 'default-policy', 1, true, p.js, p.h FROM p
  ON CONFLICT(tenant_id, name, version) DO UPDATE SET active=true, policy_json=EXCLUDED.policy_json, policy_hash=EXCLUDED.policy_hash, updated_at=now();
"

echo "[seed] Done."
echo "[seed] Use header x-umbra-tenant-id with TenantA/TenantB IDs above."
