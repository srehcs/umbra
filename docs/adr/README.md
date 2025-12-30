# Architecture Decision Records (ADRs)

This directory is the single source of truth for architectural decisions across the repo.

## How to add an ADR
1) Copy `0000-template.md` to `NNNN-short-title.md`.
2) Fill in status, date, and owners; keep the title clear and searchable.
3) Link related issues/PRs and affected docs.
4) If superseding, note it in both the new and old ADR.

## Naming & numbering
- Four-digit sequential number (e.g., `0008`).
- Lowercase, hyphenated title.

## Index
- `0001-v0-stack-go-nextjs.md` — V0 Tech Stack (Go services + Next.js UI)
- `0002-multi-tenancy-shared-db.md` — Multi-tenancy (shared Postgres, tenant-aware schema)
- `0003-receipts-hashchain-signing-ready.md` — Receipts (append-only + hash chain; signing-ready)
- `0004-deploy-model-customer-only-migrate-to-hybrid.md` — Deploy model (customer-only now; migrate to SaaS or hybrid later)
- `0005-policy-evaluator-interface.md` — Policy evaluator interface (ABAC now, swappable later)
- `0006-redis-streams-events.md` — Eventing via Redis Streams (V0 queue/event bus)
- `0007-service-to-service-auth-identity.md` — Service-to-service auth and workload identity
- `0008-pep-integration-pattern.md` — PEP integration pattern (HTTP proxy / ext_authz)
- `0009-identity-provider-claims-model.md` — Identity provider choice and claims model
