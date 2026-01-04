# Error Envelope

All HTTP APIs return a consistent error envelope:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "human-readable message",
    "details": [
      { "field": "field_name", "message": "validation message" }
    ]
  },
  "request_id": "uuid"
}
```

Notes:
- `details` is optional and only used for validation errors.
- `request_id` is always present (generated server-side if missing).

## JSON-RPC (MCP Adapter)

The MCP adapter uses JSON-RPC error responses and maps the same envelope into
`error.data`:

```json
{
  "jsonrpc": "2.0",
  "id": "rpc-id",
  "error": {
    "code": -32000,
    "message": "upstream error",
    "data": {
      "error": {
        "code": "UPSTREAM_ERROR",
        "message": "upstream error"
      },
      "request_id": "uuid",
      "decision_id": "uuid"
    }
  }
}
```

This keeps error semantics consistent across HTTP and JSON-RPC surfaces.
