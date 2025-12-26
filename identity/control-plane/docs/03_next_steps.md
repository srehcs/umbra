# Next Steps (toward a strong demo)

## Demo milestone
- Show a policy change flipping allow → deny
- Demonstrate receipts correlation (decision_id, trace_id)
- Demonstrate integrity chaining (prev_hash → hash)

## Engineering hardening (short list)
1) Enforce strict typing across API responses (no dynamic maps for public endpoints).
2) Add a uniform error envelope (code/message/request_id).
3) Add end-to-end trace propagation through PEP → PDP → receipts.
4) Add “path to OIDC” implementation plan and minimal Keycloak wiring.
5) Add signature-ready receipts: keyless (future) and local key (dev) options.
