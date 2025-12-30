# ADR-0001: V0 Tech Stack (Go services + Next.js UI)

- Status: accepted
- Date: 2025-12-26
- Owners: Umbra

## Context
We need an enterprise-adoptable, scalable stack for an identity/policy control plane with a PEP gateway component and strong observability.

## Decision
- Backend services in **Go**
- UI in **TypeScript (Next.js + shadcn/ui)** with pnpm
- Local dev via docker-compose

## Alternatives considered
- : strong safety, but slower team iteration for V0 and higher ecosystem friction for gateways.
- Java: great enterprise footprint; we may introduce Java later for specific components, but V0 favors a lean gateway + fast build pipeline.

## Consequences
- Strong fit for network services (PEP).
- Easy containers and CI.
- Keep service boundaries strict to enable future language diversification if needed.
