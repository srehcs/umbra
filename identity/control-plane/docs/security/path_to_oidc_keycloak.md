# Control-Plane Auth Posture and Path Forward

## Current state

Umbra now supports a provider-capable control-plane auth flow for local and test deployments:

- The UI exposes OIDC login/callback/logout routes.
- The callback exchanges the provider code for an access token and stores it in an HTTP-only cookie.
- `GET /api/auth/session` and `/api/controlplane/*` use the server-held token.
- The controlplane API validates JWTs and derives `tenant_id` plus roles from verified claims.
- When auth is disabled, local demo mode can still use `x-umbra-tenant-id` for tenant-scoped development flows.

Supported role claim sources today:

- `roles`
- `groups`
- `realm_access.roles`
- `resource_access.umbra.roles`

If `groups` are used, they must be exact role names or `/umbra/<role>` group paths.

## Goal

Move from a local provider-capable flow to a production-ready identity posture:

- tenant derived from a trusted claim such as `tenant_id`
- roles derived from explicit claims or scoped group paths
- admin capabilities enforced on tool/policy/receipt endpoints
- service-to-service auth for PEP → PDP and other internal hops
- documented customer IdP mapping that does not depend on Keycloak-specific behavior

## What remains

1. Publish a production-oriented IdP integration guide:
   - issuer/client configuration expectations
   - claim mapping requirements
   - JWT audience/issuer validation requirements
2. Define the production edge pattern for mTLS or certificate-auth environments:
   - do cert validation at the edge
   - hand off to Umbra using short-lived JWTs or a trusted auth broker
   - do not rely on browser-supplied identity headers in auth mode
3. Add explicit service-to-service auth for PEP → PDP.
4. Keep the local dev-header path clearly isolated from auth-enabled deployments.

## Non-goals in V0

- SCIM sync
- fine-grained relationship models
- fully packaged IdP automation for every deployment environment
