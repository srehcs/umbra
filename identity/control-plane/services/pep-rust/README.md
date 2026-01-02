# Rust PEP (pep-rust)

MCP-first enforcement service that delegates decisions to PDP and writes receipts via controlplane-api.

## Env vars

- `PEP_MODE`: `observe` or `enforce` (default: `observe`)
- `PDP_URL`: PDP base URL (default: `http://pdp:8081`)
- `CONTROLPLANE_API_URL`: Control plane API base URL (default: `http://controlplane-api:8080`)
- `MCP_UPSTREAM_URL`: MCP upstream URL (required for forwarding)
- `MCP_TENANT_ID`: tenant UUID for requests (default: `00000000-0000-0000-0000-000000000001`)
- `MCP_ACTOR_ID`: actor id (default: `user-1`)
- `MCP_ACTOR_TYPE`: actor type (default: `agent`)
- `MCP_ACTOR_ROLES`: CSV roles (default: `developer`)
- `MCP_SERVER_NAME`: MCP server name (default: `demo.mcp`)
- `PORT`: listen port (default: `8084`)

## Endpoints

- `POST /mcp` JSON-RPC MCP tool calls (`tools/call`)
- `GET /healthz` service health
