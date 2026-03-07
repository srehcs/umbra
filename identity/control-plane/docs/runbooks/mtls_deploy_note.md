# Runbook Note: mTLS Edge Deployment

Use this note alongside `docs/security/mtls.md` when validating production-like ingress behavior.

## What to validate
1) Requests without a valid client certificate are rejected at ingress.
2) Requests with a valid client certificate are forwarded with:
   - `x-umbra-user`
   - `x-umbra-roles`
   - `x-umbra-tenant-id`
3) Trace/request correlation headers are preserved (`traceparent`, `x-umbra-request-id`).
4) Umbra endpoints continue to return standard error envelopes when identity is missing/invalid.

## Quick verification checklist
- Negative path: no cert (or untrusted cert) -> ingress reject.
- Positive path: valid cert -> health/API path reachable with mapped identity context.
- Auditability path: perform one allow and one deny request, then confirm receipts include correlation IDs.

## Rollback posture
- If cert validation/mapping is unstable, disable the mTLS listener and route through last known-good ingress config.
- Do not fall back to public header-based trust from arbitrary client networks.

## Related docs
- `docs/security/mtls.md`
- `docs/security/path_to_oidc_keycloak.md`
- `docs/runbooks/pdp_unavailable.md`
