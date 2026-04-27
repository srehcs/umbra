# mTLS Front-Door Guidance (V0)

Purpose: define a production-oriented pattern for client certificate validation at the edge and safe identity handoff into Umbra's auth-enabled control plane.

## Scope

- Applies to production/staging ingress in front of the UI and Control Plane API.
- Covers high-level identity handoff expectations after certificate validation.
- Covers certificate lifecycle expectations at a policy level only.

Out of scope:

- Storing private keys in repo.
- Environment-specific certificate inventory or rotation schedules.
- Gateway-specific extraction templates or operational runbooks.

## Current auth model

When control-plane auth is enabled:

- the UI and API expect verified JWT claims, not raw browser-supplied identity headers
- the UI stores the provider access token in an HTTP-only cookie
- the UI control-plane proxy forwards `Authorization: Bearer ...` to the API
- the API derives tenant and roles from verified claims

Because of that, production ingress should not rely on direct forwarding of:

- `x-umbra-user`
- `x-umbra-roles`
- `x-umbra-tenant-id`

as the primary identity mechanism for auth-enabled UI/API traffic.

## Recommended termination model

Terminate mTLS at a trusted ingress gateway or auth broker. After certificate validation, hand identity to Umbra using a short-lived JWT or equivalent trusted token flow that Umbra can validate with issuer/audience checks.

Required controls:

1. Only the trusted gateway or auth broker can reach protected Umbra API/UI network paths.
2. Certificate validation failure must stop the request before it reaches Umbra.
3. The identity handoff to Umbra must preserve subject, tenant, and role claims in a verifiable token.
4. JWT audience and issuer must be configured and validated by both the UI server routes and controlplane API.
5. Request correlation headers (`x-umbra-request-id`, `traceparent`) must be preserved end-to-end.

## Acceptable handoff patterns

- Browser/UI path:
  - client certificate validated at the edge
  - edge or auth broker redirects into an IdP or trusted token-minting flow
  - Umbra receives a provider token and stores it in an HTTP-only cookie
- API/automation path:
  - certificate validated at the edge
  - edge or auth broker exchanges that identity for a short-lived JWT scoped to Umbra
  - Umbra validates the JWT directly

## Certificate lifecycle expectations (high-level)

- Issue client certificates from an internal CA chain controlled by platform/security.
- Maintain a documented revocation process (CRL/OCSP or equivalent platform control).
- Use bounded certificate validity and rotate according to internal policy.
- Keep key material and lifecycle runbooks outside this repository.

## Local/demo path

For local demos, Umbra still supports a non-production header-based tenant flow when auth is disabled.

Constraints:

- Local/demo only.
- Header injection is not equivalent to production identity assurance.
- Do not expose a header-trusting path to arbitrary client networks.
