# ADR-0007: Service-to-service auth and workload identity

- Status: proposed
- Date: 2025-12-30
- Owners: Umbra

## Context
Production traffic crosses multiple internal services (PEP, PDP, control-plane API).
We need strong guarantees that requests between services are authentic, authorized,
and carry a verified tenant identity.

## Decision
Adopt service-to-service authentication based on mTLS and workload identities:
- Each service presents a workload identity certificate.
- Requests between services are authenticated via mTLS.
- Tenant identity is passed explicitly (header + trace attributes) and validated
  at ingress against the calling workload identity.

## Alternatives considered
- Shared API keys: easy but weak, poor auditability, hard to rotate.
- Network-level trust only: insufficient for multi-tenant and compliance needs.

## Consequences
- Requires PKI or workload identity provider integration.
- Additional connection setup and certificate rotation logic.
- Enables strong audit trails and better isolation guarantees.
