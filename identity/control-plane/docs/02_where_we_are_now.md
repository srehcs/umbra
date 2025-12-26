# Where We Are Now (V0 State)

This document is a reality check: **what is implemented in this repo** vs **what is planned**.

## Implemented
- **Local stack** via docker-compose: Postgres, Redis (optional), OTel Collector, Jaeger, services, UI.
- **Control Plane API**:
  - tools CRUD (tenant-scoped)
  - policies CRUD + activation (tenant-scoped)
  - receipts list + cursor pagination (tenant-scoped)
- **PDP**:
  - `/v1/decision` evaluates ABAC policy (default-deny)
  - writes decision receipts (hash-chained, signing-ready)
- **PEP Gateway (HTTP)**:
  - demo enforcement route that calls PDP before forwarding
  - writes invocation receipts (hash-chained, signing-ready)
- **UI (Next.js + ShadCN)**:
  - tools
  - policies (author + activate)
  - receipts table + structured detail view

## Planned (documented, not fully implemented)
- **MCP adapter**: intercept MCP tool calls and convert to PDP requests.
- **CLI wrapper**: capture tool invocations and convert to PDP requests + receipts.
- **OIDC / Keycloak**: replace dev tenant header with claims-based tenancy and RBAC.
- **Signature verification**: keep hash-chain; add signing keys and verification pipeline.

## Known hardening items (near-term)
1) Make Go transport objects fully typed end-to-end (Decision request/response and receipts variants).
2) Uniform error envelopes and consistent status codes.
3) Stronger policy validation + simulation in control-plane API (not only UI).
4) Service-to-service auth for PEP → PDP.
