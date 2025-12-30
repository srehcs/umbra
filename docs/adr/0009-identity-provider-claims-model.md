# ADR-0009: Identity provider choice and claims model

- Status: proposed
- Date: 2025-12-30
- Owners: Umbra

## Context
Control-plane access and tenant identity rely on trusted authentication.
We need a default IdP and a clear claims model for tenant scoping.

## Decision
Use Keycloak OIDC as the default IdP for V0:
- Required claims: `sub`, `tenant_id`, and role/permission claims.
- Tenant identity is derived from a trusted IdP claim and propagated to services.

## Alternatives considered
- Custom auth: higher implementation risk and slower adoption.
- Vendor-specific IdPs only: reduces portability for customers.

## Consequences
- Requires documented claim mapping and tenant provisioning workflow.
- Keeps an escape hatch for customer IdPs by mapping claims to the same model.
