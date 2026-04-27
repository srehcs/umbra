# Umbra UI (Next.js)

A high-signal V0 console for the Agent Identity Control Plane.

## What’s in the UI

- App shell + navigation
- Tenant switcher for dev-mode tenant scoping
- Tools CRUD
- Policies CRUD + activation + **validate + simulate**
- Receipts table with **server-side filtering + cursor pagination**
- Receipt detail: **structured view + raw JSON**
- OIDC login/callback/logout routes with cookie-backed session support when auth is enabled

## Local dev

```bash
pnpm install
pnpm dev
```

## Tenant context and auth

When auth is disabled for local demos, the UI stores the tenant UUID in localStorage and sends it as:

- `x-umbra-tenant-id`

Use `make seed` at repo root to print TenantA/TenantB IDs, then set it in the sidebar.

When auth is enabled:

- the UI reads session state from `/api/auth/session`
- the server stores the provider access token in an HTTP-only cookie
- `/api/controlplane/*` forwards `Authorization: Bearer ...` server-side
- tenant and roles are derived from verified token claims

## Environment

- `CONTROLPLANE_API_URL` for the server-side proxy target (default: `http://localhost:8080` in local dev, `http://controlplane-api:8080` in Docker Compose)
- `NEXT_PUBLIC_AUTH_ENABLED` to enable client-side auth mode
- `UMBRA_AUTH_ENABLED` to enable server-side auth routes (`AUTH_ENABLED` is accepted for compatibility)
- `NEXT_PUBLIC_AUTH_DEV_TOKEN_ENABLED` only for explicit local dev-token fallback testing
