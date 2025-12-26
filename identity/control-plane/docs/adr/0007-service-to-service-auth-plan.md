# ADR-0007: Service-to-service auth plan (stub for V0)

- Status: proposed
- Date: 2025-12-26
- Owners: Umbra

## Context
In production, PEP→PDP and UI/API interactions must be authenticated and authorized. V0 uses
local/dev trust with future hardening planned.

## Decision (plan)
- Use mTLS for service-to-service calls (PEP→PDP, controlplane→PDP if needed).
- Introduce workload identities (SPIFFE/SPIRE or cloud-native equivalents).
- PDP enforces caller identity: only authorized PEP instances can call `/v1/decision`.

## Alternatives
- Shared API keys (fragile, secret management burden)
- Network-only trust (insufficient)

## Consequences
- V0 demo remains simple
- Pilot readiness requires implementing this ADR
