# Postgres RLS Plan (to move from "No" → "Yes")

You chose **NO** for Postgres Row-Level Security (RLS) in V0.
This doc outlines what must change to enable RLS safely later.

## Why enable RLS?
RLS provides defense-in-depth against cross-tenant data leakage by enforcing tenant predicates at the database layer.

## Preconditions
- All tables already include `tenant_id` (this scaffold does).
- Every DB connection must set the tenant context (e.g., `SET app.tenant_id = '...'`).
- All queries must be compatible with RLS predicates (no cross-tenant admin queries without explicit bypass role).

## Required changes
1) **DB roles**
   - Create an application role with minimal privileges.
   - Create an elevated admin role for maintenance tasks.

2) **Session tenant variable**
   - On connection checkout, execute:
     - `SET LOCAL app.tenant_id = '<tenant_uuid>';`
   - Ensure transaction boundaries are clear (use per-request transactions or explicit reset).

3) **Enable RLS per table**
   - `ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;`
   - `CREATE POLICY tenant_isolation ON <t> USING (tenant_id::text = current_setting('app.tenant_id'));`

4) **Migrations + tests**
   - Add migrations to enable RLS and policies.
   - Add integration tests that prove tenant A cannot read tenant B across all read paths.

5) **Control plane exceptions**
   - If you need cross-tenant “super-admin”, use a different DB role and explicit code path + audit receipts.

## Rollout plan
- Stage 1: enable RLS in dev with feature flag
- Stage 2: run shadow tests in staging
- Stage 3: enable in production tenant-by-tenant

## Risks
- Forgetting to set tenant context → queries return empty or error.
- Performance impact if indexes are missing on `tenant_id`.

## Index requirement
Ensure every tenant-aware table has an index starting with `tenant_id`.
