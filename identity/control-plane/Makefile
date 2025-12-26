SHELL := /bin/bash

COMPOSE := docker compose -f deployments/docker-compose.yml

.PHONY: bootstrap fmt lint test gen up down logs dev seed docker-build verify migrate verify

bootstrap:
	@echo "Bootstrap: go mod tidy (optional) + ui deps"
	@cd ui && pnpm install || true

fmt:
	@gofmt -w .
	@cd ui && pnpm format --if-present || true

lint:
	@echo "Lint (V0): go vet + ui lint"
	@go vet ./...
	@cd ui && pnpm lint --if-present || true

test:
	@go test ./...

gen:
	@echo "Generate (V0): OpenAPI → TS types (stub)"
	@cd ui && pnpm gen --if-present || true

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=200

seed:
	@bash scripts/dev/seed_db.sh

dev: up seed
	@echo "Local stack up. See README for ports."

docker-build:
	$(COMPOSE) build


migrate:
	@docker compose -f deployments/docker-compose.yml exec -T postgres psql -U umbra -d umbra -v ON_ERROR_STOP=1 -f /migrations/0001_init.sql

verify:
	@bash scripts/dev/verify_local.sh

verify:
	@bash scripts/dev/verify_local.sh
