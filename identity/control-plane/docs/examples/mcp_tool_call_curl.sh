#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${MCP_URL:-http://localhost:8083/mcp}"
REQUEST_ID="${REQUEST_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"

# If actor metadata is omitted, the adapter uses a deterministic dev identity (actor.source=dev).
# If roles are omitted, receipts store an empty roles array. Raw args are redacted from receipts.

curl -sS -X POST "${MCP_URL}" \
  -H "content-type: application/json" \
  -d '"'"'{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "demo.tool",
      "arguments": {"path": "/demo", "method": "GET"},
      "server": "demo.mcp",
      "workspace": "demo-workspace",
      "actor": {"id": "user-1", "type": "human", "roles": ["developer"], "source": "client"},
      "request_id": "'"'"'${REQUEST_ID}'"'"'"
    }
  }'"'"' | jq .
