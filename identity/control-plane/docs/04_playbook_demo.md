# Demo Playbook

## Prereqs
- Docker + Docker Compose
- Go toolchain and Node.js + pnpm (for UI if running outside compose)

## Start stack
From `identity/control-plane/`:

```bash
make dev
make seed
```

## Demo script
1) Open UI: http://localhost:3000
2) Tools → create a tool definition (name + endpoint)
3) Policies → create policy JSON (ABAC V0) and activate
4) ## Demo script
1) Open UI: http://localhost:3000
2) Tools → create a tool definition (name + endpoint)
3) Policies → create policy JSON (ABAC V0) and activate
4) Invoke through **HTTP PEP gateway** (V0):
   - PEP: http://localhost:8082
   - Example request: `docs/examples/pep_invoke_curl.sh`
5) Receipts → verify:
   - decision receipts show allow/deny + policy_hash
   - invocation receipts show outcome + latency
   - hash chain links via prev_hash

## Examples
- `docs/examples/pep_invoke_curl.sh`
- `docs/examples/pdp_decision_curl.sh`
