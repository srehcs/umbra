# Threat Model (V0 skeleton)

## Key assets
- Policy decisions and obligations
- Receipts (audit integrity)
- Tenant isolation
- Credentials (future brokered tokens)

## Primary threats (STRIDE-ish)
- Spoofing: fake agent identity / tenant claim
- Tampering: receipt modification / policy changes without audit trail
- Repudiation: lack of evidence for decisions/tool invocations
- Information disclosure: cross-tenant leakage; logging secrets
- Denial of service: PDP unavailable; PEP overload
- Elevation of privilege: auth bypass; policy mis-evaluation

## V0 mitigations
- Default-deny policy evaluation semantics
- Request bounds + timeouts
- Tenant_id required in DB queries
- Structured logs with redaction policy
- Hash-chained receipts

## Follow-ups
- Service-to-service auth (mTLS + workload IDs)
- Optional Postgres RLS (see `docs/security/rls_plan.md`)
