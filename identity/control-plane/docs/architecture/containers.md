# Containers (C4 L2)

- **PEP Gateway**: intercepts tool calls (HTTP proxy/ext_authz style). Calls PDP.
  - Includes MCP adapter stub and CLI wrapper stub.
- **PDP**: evaluates policy (V0 ABAC) and returns allow/deny + obligations.
- **Control Plane API**: manages tenants, tools, policies; queries audit receipts.
- **Postgres**: source-of-truth for config + receipts.
- **Redis**: caching + queue/event bus (V0 uses Streams as placeholder).

Observability:
- OTel collector + trace backend (Jaeger in local dev).
