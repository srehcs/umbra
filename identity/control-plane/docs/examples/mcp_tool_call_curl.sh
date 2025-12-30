#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${MCP_URL:-http://localhost:8083/mcp}"
REQUEST_ID="${REQUEST_ID:-$(uuidgen | tr '[:upper:]' '[:lower:]')}"

curl -sS -X POST "${MCP_URL}" \
  -H "content-type: application/json" \
  -d '"'"'{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "demo.tool",
      "arguments": {"path": "/demo", "method": "GET"},
      "request_id": "'"'"'${REQUEST_ID}'"'"'"
    }
  }'"'"' | jq .
