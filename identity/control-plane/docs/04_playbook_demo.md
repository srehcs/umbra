# MCP Demo Playbook (0009)

This demo shows MCP-first enforcement with allow/deny outcomes and receipts you can verify in the UI.

## Prerequisites
- Docker + Docker Compose
- Go toolchain (for local builds if needed)
- `jq` (optional, for pretty output)

## Start the stack
From `identity/control-plane`:

```bash
make dev
make seed
```

`make seed` prints tenant IDs. Pick `TenantA` and export it:

```bash
export TENANT_ID="<tenant-a-uuid>"
export MCP_URL="http://localhost:8083/mcp"
```

## Configure the MCP client (primary: minimal JSON-RPC client)
We provide a minimal curl-based MCP client with request payloads:
- `deployments/mcp/deny.json`
- `deployments/mcp/allow.json`

These include actor + server metadata so the policy rules match deterministically.

## Example tool calls
### 1) Deny (sensitive tool)
```bash
curl -sS -X POST "$MCP_URL" \
  -H "content-type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d @deployments/mcp/deny.json | jq .
```

### 2) Allow (safe tool)
```bash
curl -sS -X POST "$MCP_URL" \
  -H "content-type: application/json" \
  -H "x-umbra-tenant-id: $TENANT_ID" \
  -d @deployments/mcp/allow.json | jq .
```

Notes:
- Default `PEP_MODE` is `observe`, so denied calls are still forwarded but receipts show `outcome=denied`.
- To block denied calls, set `PEP_MODE=enforce` for `mcp-adapter` in `deployments/docker-compose.yml` and restart `make dev`.

## Where to view receipts
- UI receipts list: http://localhost:3000/receipts
- Receipt detail shows decision + correlation IDs (request_id, decision_id) and policy hash.

For hash-chain fields, use the API:
```bash
curl -sS -H "x-umbra-tenant-id: $TENANT_ID" \
  "http://localhost:8080/v1/receipts?kind=invocation" | jq .
```
`body.hash` and `body.prev_hash` (when present) confirm chaining.

## Known-good demo policy
`make seed` loads a policy with:
- **deny**: MCP tool `demo.secret` for `tools/call` on server `demo.mcp`
- **allow**: MCP tool `demo.safe` for `tools/call` on server `demo.mcp`
- **allow**: HTTP demo `GET /demo` for roles `admin` or `developer`

Policy JSON (excerpt):
```json
{
  "version": 1,
  "mode": "abac_v0",
  "rules": [
    {
      "effect": "deny",
      "mcp_servers_any": ["demo.mcp"],
      "mcp_tools_any": ["demo.secret"],
      "mcp_methods_any": ["tools/call"]
    },
    {
      "effect": "allow",
      "mcp_servers_any": ["demo.mcp"],
      "mcp_tools_any": ["demo.safe"],
      "mcp_methods_any": ["tools/call"]
    },
    {
      "effect": "allow",
      "roles_any": ["admin", "developer"],
      "methods_any": ["GET"],
      "path_prefix": "/demo"
    }
  ],
  "default": "deny"
}
```

## Expected results
- First call: denied (blocked in enforce mode, recorded in observe mode)
- Second call: allowed and forwarded
- Receipts show allow/deny outcomes, correlation IDs, and hash chain fields via API
