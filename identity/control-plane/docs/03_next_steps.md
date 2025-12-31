# Next Steps (toward a strong demo)

## Demo milestone
- Show a policy change flipping allow → deny
- Demonstrate receipts correlation (request_id, decision_id, trace_id)
- Demonstrate integrity chaining (prev_hash → hash)

## Engineering hardening (short list)
1) Enforce strict typing across API responses (no dynamic maps for public endpoints).
2) Add a uniform error envelope (code/message/request_id).
3) Add end-to-end trace propagation through PEP → PDP → receipts (trace_id + span_id stored).
4) Add “path to OIDC” implementation plan and minimal Keycloak wiring.
5) Add signature-ready receipts: keyless (future) and local key (dev) options.

## Contract coverage (status)
- OpenAPI now includes receipts POST/verify, receipts export, and policies active endpoints.
- Error responses document `x-umbra-request-id` header and a flexible error envelope shape.
- Keep contract updates aligned with service behavior and UI clients.
