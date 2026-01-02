SHELL := /bin/bash

COMPOSE := docker compose -f deployments/docker-compose.yml

.PHONY: bootstrap fmt lint test gen up down logs dev seed docker-build verify migrate receipts-verify e2e-setup e2e e2e-fast e2e-local

bootstrap:
	@echo "Bootstrap: go mod tidy (optional) + ui deps"
	@cd ui && pnpm install || true

fmt:
	@gofmt -w .
	@cd ui && pnpm --if-present format || true

lint:
	@echo "Lint (V0): go vet + ui lint"
	@go vet ./...
	@cd ui && pnpm --if-present lint || true

test:
	@go test ./...

gen:
	@echo "Generate (V0): OpenAPI → TS types"
	@cd ui && pnpm --if-present gen

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=200

seed:
	@bash scripts/dev/seed_db.sh

# Start local stack, wait for readiness, seed the DB, and print endpoints
dev:
	@mkdir -p ./local/.data/postgres
	$(COMPOSE) up -d --build
	@bash scripts/dev/wait_for_services.sh
	@bash scripts/dev/seed_db.sh
	@echo "\nLocal endpoints:"
	@echo "UI:  http://localhost:3000"
	@echo "API: http://localhost:8080"
	@echo "PDP: http://localhost:8081"
	@echo "PEP: http://localhost:8082"

# Check demo readiness (db, api, pdp, tools/policies, receipts)
.PHONY: demo-check
demo-check:
	@bash scripts/dev/demo_check.sh

docker-build:
	$(COMPOSE) build

migrate:
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0001_init.sql
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0002_add_request_id.sql
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0003_add_receipt_indexes.sql
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0004_add_receipt_search_indexes.sql
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0005_add_receipt_search_text.sql
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0006_add_receipt_canonical.sql

verify:
	@bash scripts/dev/verify_local.sh

# Verify receipt hash chains (decision/invocation) via API.
receipts-verify:
	@TENANT_ID=$${TENANT_ID:-} ; \
	if [ -z "$$TENANT_ID" ]; then \
		echo "TENANT_ID is required (set it to the seeded tenant UUID)"; \
		exit 1; \
	fi ; \
	KIND=$${KIND:-all} ; \
	LIMIT=$${LIMIT:-100} ; \
	curl -s -X POST -H "x-umbra-tenant-id: $$TENANT_ID" "http://localhost:8080/v1/receipts/verify?kind=$$KIND&limit=$$LIMIT" | jq .

e2e-setup:
	@bash scripts/dev/e2e_setup.sh

e2e:
	@bash scripts/dev/e2e_setup.sh
	@cd ui && pnpm test:e2e

# Reuse existing images (skip docker --build)
e2e-fast:
	@E2E_NO_BUILD=1 bash scripts/dev/e2e_setup.sh
	@cd ui && pnpm test:e2e

# Assume stack already running; skip docker compose up
e2e-local:
	@E2E_SKIP_UP=1 bash scripts/dev/e2e_setup.sh
	@cd ui && pnpm test:e2e
