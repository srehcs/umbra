# Trust Boundaries

1) **Ingress boundary**: PEP receives tool invocations from agent runtimes.
2) **Decision boundary**: PEP → PDP decision call (must be authenticated/authorized in production).
3) **Control plane boundary**: Admin UI/API protected by OIDC; tenant-scoped.
4) **Data boundary**: Postgres contains tenant-separated data; strict query hygiene required.
5) **Telemetry boundary**: OTel pipeline must not leak secrets; receipts are metadata-only.

V0: mTLS and service-to-service auth is stubbed; ADR required before production.
