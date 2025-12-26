# Umbra UI (Next.js)

A high-signal V0 console for the Agent Identity Control Plane.

## What’s in the UI
- App shell + navigation
- Tenant switcher (V0 dev-mode header)
- Tools CRUD
- Policies CRUD + activation + **validate + simulate**
- Receipts table with **server-side filtering + cursor pagination**
- Receipt detail: **structured view + raw JSON**

## Local dev
```bash
pnpm install
pnpm dev
```

## Tenant context (V0)
The UI stores the tenant UUID in localStorage and sends it as:
- `x-umbra-tenant-id`

Use `make seed` at repo root to print TenantA/TenantB IDs, then set it in the sidebar.

## Environment
- `NEXT_PUBLIC_CONTROLPLANE_URL` (default: http://localhost:8080)
