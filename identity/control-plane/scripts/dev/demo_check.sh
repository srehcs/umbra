#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"

echo "[demo-check] Running docker-compose config check"
if ! docker compose -f deployments/docker-compose.yml config >/dev/null 2>&1; then
  echo "docker compose config failed"
  exit 1
fi

echo "[demo-check] Checking database connectivity"
if ! $COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT 1" >/dev/null 2>&1; then
  echo "[demo-check] Postgres not reachable"
  exit 1
fi

echo "[demo-check] Verifying receipts table exists"
RECEIPTS_EXISTS=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT to_regclass('public.receipts_invocation') IS NOT NULL;")
RECEIPTS_EXISTS=$(echo "$RECEIPTS_EXISTS" | xargs)
if [[ "$RECEIPTS_EXISTS" != "t" ]]; then
  echo "[demo-check] receipts_invocation table not present"
  exit 1
fi

echo "[demo-check] Checking service health endpoints"
for url in "http://localhost:8080/healthz" "http://localhost:8081/healthz" "http://localhost:8082/healthz"; do
  if ! curl -sSf "$url" >/dev/null 2>&1; then
    echo "[demo-check] $url not healthy"
    exit 1
  fi
done

echo "[demo-check] Checking at least one tenant exists"
TENANT_COUNT=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT count(*) FROM tenants;" | xargs)
if [[ -z "$TENANT_COUNT" || "$TENANT_COUNT" -lt 1 ]]; then
  echo "[demo-check] No tenants found"
  exit 1
fi

# Use the first tenant id
TENANT_ID=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tA -c "SELECT id FROM tenants ORDER BY created_at ASC LIMIT 1;" | xargs)
if [[ -z "$TENANT_ID" ]]; then
  echo "[demo-check] Failed to read tenant id"
  exit 1
fi

echo "[demo-check] Checking tools/policies for tenant $TENANT_ID"
TOOLS_COUNT=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT count(*) FROM tools WHERE tenant_id='$TENANT_ID';" | xargs)
POLICIES_COUNT=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT count(*) FROM policies WHERE tenant_id='$TENANT_ID';" | xargs)

if [[ -z "$TOOLS_COUNT" || "$TOOLS_COUNT" -lt 1 ]]; then
  echo "[demo-check] No tools found for tenant $TENANT_ID"
  exit 1
fi
if [[ -z "$POLICIES_COUNT" || "$POLICIES_COUNT" -lt 1 ]]; then
  echo "[demo-check] No policies found for tenant $TENANT_ID"
  exit 1
fi

echo "[demo-check] Querying receipts table for basic query"
RC=$($COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT count(*) FROM receipts_invocation;" | xargs)
if [[ -z "$RC" ]]; then
  echo "[demo-check] Failed to query receipts"
  exit 1
fi

echo "[demo-check] All checks passed"
exit 0