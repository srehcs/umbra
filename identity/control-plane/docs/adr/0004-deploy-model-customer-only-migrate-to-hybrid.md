# ADR-0004: Deploy model (customer-only now; migrate to SaaS or hybrid later)

- Status: accepted
- Date: 2025-12-26
- Owners: Umbra

## Context
Enterprises adopt faster when sensitive traffic and enforcement run inside their environment.
We also want a path to sell a hosted control plane later.

## Decision
- V0 targets **customer-only** deployment in a **customer VPC** (or on-prem if required), using docker-compose for local dev.
- The architecture and data model remain compatible with a **hosted control plane** later (SaaS) and/or **hybrid**: customer-hosted PEP + hosted PDP/control-plane.

## Migration path to hybrid
- Split “control-plane API + PDP” into a hosted control plane (SaaS).
- Keep PEP in customer environment; PEP talks to hosted PDP via mTLS and tenant-scoped identity.
- Add outbound-only connectivity options, customer-controlled allowlists, and explicit data minimization.
- Add per-tenant encryption, key management, and stronger isolation (RLS / per-tenant DB) if needed.

## Consequences
- V0 local demos match enterprise reality.
- Hybrid becomes a deployment choice rather than a redesign.
