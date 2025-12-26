# Executive Summary

Umbra's **Agent Identity Control Plane (V0-C)** is an enterprise-facing control plane that sits at the **tool/action boundary** for agentic systems.

It provides:
- **Policy decisions** (PDP): allow/deny + obligations for tool invocations.
- **Enforcement** (PEP): interception at MCP and CLI boundaries (and extensible to HTTP proxy/ext_authz).
- **Audit receipts**: hash-chained, signing-ready records correlated with traces (OpenTelemetry IDs).

V0-C goal: a credible, end-to-end demo that shows an enterprise-safe posture:
1) register a tool,
2) author/activate a policy,
3) run a tool invocation through PEP,
4) see decision + invocation receipts in the UI, with integrity chaining.

Non-goals in V0:
- full OIDC/Keycloak integration (stubbed with an explicit “path to yes” doc),
- distributed event bus / streaming pipeline,
- multi-region HA and advanced policy languages.

Success criteria:
- deterministic evaluation semantics,
- evidence-grade receipts,
- strict boundaries between services and shared packages,
- a clean upgrade path to OIDC, signing, and more advanced policy engines.
