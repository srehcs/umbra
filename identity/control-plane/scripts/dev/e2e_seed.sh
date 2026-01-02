#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"
TENANT_ID="${E2E_TENANT_ID:-11111111-1111-1111-1111-111111111111}"
TENANT_NAME="${E2E_TENANT_NAME:-E2E}"

echo "[e2e] Seeding deterministic tenant $TENANT_NAME ($TENANT_ID)"

cat <<PSQL | $COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1
  INSERT INTO tenants(id, name)
  VALUES ('$TENANT_ID', '$TENANT_NAME')
  ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name;
PSQL

cat <<PSQL | $COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1
  INSERT INTO tools(tenant_id, name, kind, config_json)
  VALUES
    ('$TENANT_ID', 'sample-http-tool', 'http', '{"upstream":"http://upstream-sample:9000"}'::jsonb)
  ON CONFLICT(tenant_id, name) DO UPDATE SET config_json=EXCLUDED.config_json, updated_at=now();
PSQL

POLICY_JSON='{"version":1,"mode":"abac_v0","rules":[{"effect":"allow","roles_any":["admin","developer"],"methods_any":["GET"],"path_prefix":"/demo"}],"default":"deny"}'

$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -c "
  WITH p AS (
    SELECT '$POLICY_JSON'::jsonb AS js, encode(digest('$POLICY_JSON','sha256'),'hex') AS h
  )
  INSERT INTO policies(tenant_id, name, version, active, policy_json, policy_hash)
  SELECT '$TENANT_ID', 'default-policy', 1, true, p.js, p.h FROM p
  ON CONFLICT(tenant_id, name, version) DO UPDATE SET active=true, policy_json=EXCLUDED.policy_json, policy_hash=EXCLUDED.policy_hash, updated_at=now();
"

echo "[e2e] Seed complete"
