# Path to OIDC/Keycloak ("No now, documented Yes")

V0 uses a tenant header (`x-umbra-tenant-id`) for development and demo purposes.

## Goal
Replace header-based tenancy with Keycloak-backed OIDC:
- tenant derived from a claim (e.g., `tenant_id`)
- roles derived from groups/roles (e.g., `roles`, `groups`)
- enforce admin capabilities on tool/policy endpoints
- support service-to-service auth for PEP → PDP

## Steps
1) Stand up Keycloak in docker-compose:
   - realm: `umbra`
   - clients: `ui`, `controlplane-api`, `pep-gateway`
2) UI:
   - NextAuth OIDC provider
   - store/access token; attach `Authorization: Bearer`
3) APIs:
   - middleware verifies JWT (issuer/audience/exp)
   - extract tenant + roles from claims
4) Authorization:
   - RBAC: `policy_admin`, `tool_admin`, `auditor`
   - enforce per endpoint
5) Telemetry:
   - attach `umbra.actor_id`, `umbra.tenant_id`, `umbra.roles` attributes
6) Backward compatible migration:
   - allow header in dev mode only, behind env flag

## Non-goals
- SCIM sync in V0
- fine-grained relationship model in V0
