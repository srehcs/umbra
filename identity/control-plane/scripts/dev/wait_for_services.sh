#!/usr/bin/env bash
set -euo pipefail

COMPOSE="docker compose -f deployments/docker-compose.yml"

echo "[wait] Ensuring required host directories exist"
mkdir -p ./local/.data/postgres

wait_for_postgres() {
  echo "[wait] Waiting for postgres..."
  for i in {1..60}; do
    if $COMPOSE exec -T postgres psql -U umbra -d umbra -tAc "SELECT 1" >/dev/null 2>&1; then
      echo "[wait] Postgres ready"
      return 0
    fi
    sleep 1
  done
  echo "[wait] Postgres did not become ready"
  return 1
}

wait_for_http() {
  local url="$1"
  local name="$2"
  echo "[wait] Waiting for $name ($url)..."
  for i in {1..60}; do
    if curl -sSf "$url" >/dev/null 2>&1; then
      echo "[wait] $name ready"
      return 0
    fi
    sleep 1
  done
  echo "[wait] $name did not become ready"
  return 1
}

# Wait for core infra
wait_for_postgres
wait_for_http "http://localhost:8080/healthz" "controlplane-api"
wait_for_http "http://localhost:8081/healthz" "pdp"
wait_for_http "http://localhost:8082/healthz" "pep-gateway"

# UI may take a bit more depending on build; check it but don't fail the overall startup if UI not ready yet
if curl -sSf "http://localhost:3000" >/dev/null 2>&1; then
  echo "[wait] UI ready"
else
  echo "[wait] UI not yet ready (it may still be building). Proceeding..."
fi

# Print endpoints for convenience
echo "\n[wait] Local endpoints:"
echo "UI:  http://localhost:3000"
echo "API: http://localhost:8080"
echo "PDP: http://localhost:8081"
echo "PEP: http://localhost:8082"

echo "[wait] Services appear to be ready"