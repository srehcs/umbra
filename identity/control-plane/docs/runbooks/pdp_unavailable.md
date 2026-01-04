# Runbook: PDP Unavailable / Timeouts

## Summary
When the PDP is unreachable or timing out, the PEP enforces a fail-closed posture in `enforce` mode and forwards in `observe` mode. Receipts record `pdp.status` and `enforcement.outcome`.

## Detect
- PEP logs include `pdp decide failed` with `status` and error details.
- Receipts:
  - `pdp.status` is `timeout` or `unavailable`
  - `enforcement.outcome` is `blocked` (enforce) or `forwarded` (observe)
- Metrics/spans: elevated latency and error rates on `pep.enforce`.

## Immediate actions
1) Confirm PDP health (service status, logs, and connectivity).
2) Verify database connectivity for PDP (if PDP uses DB-backed policy).
3) Check network policies/mTLS if in production mode.

## Safe toggles and risks
- Set `PEP_MODE=observe` to keep traffic flowing during an incident.
  - Risk: enforcement is bypassed, allowing potentially unsafe tool calls.
- Adjust PDP timeout (if configured) to reduce false timeouts during load spikes.
  - Risk: longer timeouts increase tail latency and can cascade under load.

## Expected client behavior
- `enforce` mode: requests return `503` with `error.code=POLICY_UNAVAILABLE`.
- `observe` mode: requests are forwarded, but receipts capture PDP unavailability.

## Recovery checklist
1) Restore PDP availability (service restart, DB health, network).
2) Validate PDP responses via `/v1/decision` (manual curl or health check).
3) Switch back to `PEP_MODE=enforce` once stable.
4) Verify receipts show `pdp.status=ok` and `enforcement.outcome=forwarded`.
