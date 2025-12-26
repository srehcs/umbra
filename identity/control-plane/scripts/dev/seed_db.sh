#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"

echo "[seed] Applying migrations..."
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0001_init.sql

echo "[seed] Seeding tenants..."
TENANT_A=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tA -c "INSERT INTO tenants(name) VALUES('TenantA') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id;")
TENANT_B=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tA -c "INSERT INTO tenants(name) VALUES('TenantB') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id;")

echo "[seed] TenantA=$TENANT_A"
echo "[seed] TenantB=$TENANT_B"

echo "[seed] Seeding tools..."
$COMPOSE exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -c "
  INSERT INTO tools(tenant_id, name, kind, config_json)
  VALUES
    ('$TENANT_A', 'sample-http-tool', 'http', '{"upstream":"http://upstream-sample:9000"}'::jsonb),
    ('$TENANT_B', 'sample-http-tool', 'http', '{"upstream":"http://upstream-sample:9000"}'::jsonb)
  ON CONFLICT(tenant_id, name) DO UPDATE SET config_json=EXCLUDED.config_json, updated_at=now();
"

echo "[seed] Seeding policies..."
# Minimal ABAC policy: allow GET /demo for role 'admin' or 'developer'
POLICY_JSON='{
  "version": 1,
  "mode": "abac_v0",
  "rules": [
    { "effect": "allow", "roles_any": ["admin","developer"], "methods_any": ["GET"], "path_prefix": "/demo" }
  ],
  "default": "deny"
}'

# compute policy_hash in SQL using sha256 over canonical text representation
# In V0 seed, we hash the JSON string bytes as provided (policy updates in API compute hash in code).
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
