# Contracts

Generate TypeScript types and a client from `docs/api/openapi.yaml`.

From `identity/control-plane/ui`:
```bash
pnpm gen
```

This writes to:
- `identity/control-plane/packages/contracts/openapi.ts`

Runtime client helper:
- `identity/control-plane/packages/contracts/client.ts` (openapi-fetch)
