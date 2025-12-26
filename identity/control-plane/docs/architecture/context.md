# System Context (C4 L1)

**System:** Umbra Agent Identity Control Plane (V0-C)

**Actors**
- Human user (developer/employee)
- Agent runtime (IDE agent / CLI agent / CI agent)
- Tool providers (internal APIs, SaaS APIs, cloud APIs)

**External systems**
- Keycloak (OIDC)
- SIEM / log archive (via OTel/log sink)
- Customer network + tool backends

**Core idea**
All tool invocations flow through a PEP, which asks the PDP for a decision and emits receipts.
