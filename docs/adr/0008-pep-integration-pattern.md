# ADR-0008: PEP integration pattern (HTTP proxy / ext_authz)

- Status: accepted
- Date: 2025-12-30
- Owners: Umbra

## Context
Tool invocations must be intercepted consistently across agent runtimes and
integrations. We need a pattern that works across languages and environments.

## Decision
Implement the PEP as an HTTP proxy/ext_authz-style gateway:
- Tool calls flow through the PEP as HTTP requests.
- The PEP calls the PDP for allow/deny + obligations.
- Receipts are emitted at the PEP boundary.

## Alternatives considered
- In-process SDK enforcement: language-specific and harder to audit uniformly.
- Sidecar per tool: higher operational overhead and inconsistent rollout.

## Consequences
- Clear, centralized enforcement and audit boundary.
- Requires routing all tool traffic through the gateway.
