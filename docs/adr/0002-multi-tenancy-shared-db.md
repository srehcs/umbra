# ADR-0002: Multi-tenancy (shared Postgres, tenant-aware schema)

- Status: accepted
- Date: 2025-12-26
- Owners: Umbra

## Context
We want true multi-tenancy from day 1 (shared DB), but remain deployable in customer-only environments and migratable to hybrid later.

## Decision
- Every data model table includes `tenant_id` (UUID).
- Tenant is derived from a trusted identity claim (Keycloak OIDC) for controlplane-api.
- For internal service calls, tenant is carried explicitly (header + trace attrs), validated at ingress.
- Data access helpers require `tenant_id` as a parameter (no “ambient” global tenant).

## Alternatives considered
- Single-tenant only: simpler, but painful migration.
- Per-tenant DB: increases ops overhead and complicates local dev.

## Consequences
- Requires strict query hygiene and tests to prevent cross-tenant data leakage.
- Enables future RLS adoption with minimal schema changes.
  - See `docs/security/rls_plan.md` for the staged rollout plan.
