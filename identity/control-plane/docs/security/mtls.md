# mTLS Front-Door Guidance (V0)

Purpose: define a production-ready pattern for client certificate validation at the edge and trusted identity header forwarding to Umbra services.

## Scope
- Applies to production/staging ingress in front of UI and Control Plane API.
- Covers certificate-to-header mapping for:
  - `x-umbra-user`
  - `x-umbra-roles`
  - `x-umbra-tenant-id`
- Covers high-level certificate lifecycle expectations.

Out of scope:
- Storing private keys in repo.
- Environment-specific certificate inventory or rotation schedules.

## Recommended termination model
Terminate mTLS at a trusted ingress gateway (Nginx/Envoy/Ingress Controller), then forward validated identity headers to Umbra.

Required controls:
1) Only the trusted gateway can reach Umbra API/UI network paths.
2) Gateway must reject requests when client cert validation fails.
3) Umbra services should treat forwarded identity headers as trusted only from gateway networks.
4) Request correlation headers (`x-umbra-request-id`, `traceparent`) must be preserved end-to-end.

## Certificate lifecycle expectations (high-level)
- Issue client certificates from an internal CA chain controlled by platform/security.
- Maintain a documented revocation process (CRL/OCSP or equivalent platform control).
- Use bounded certificate validity and rotate according to internal policy.
- Keep key material and lifecycle runbooks outside this repository.

## Certificate-to-header mapping
Use a deterministic mapping profile in gateway config and keep it consistent across environments.

Suggested mapping profile:
- `x-umbra-user` <- certificate subject CN or approved SAN identifier
- `x-umbra-roles` <- role attribute from cert subject/extension (comma-separated after normalization)
- `x-umbra-tenant-id` <- tenant UUID attribute from SAN/extension mapped by policy

Validation requirements:
- Reject empty or malformed mapped values.
- Enforce tenant ID format before forwarding.
- Normalize role delimiters and allowed role set.

## Nginx reference config (illustrative)
Use placeholder paths and adapt extraction logic to your PKI profile.

```nginx
# NOTE: illustrative only; adapt cert field extraction to your cert profile.
server {
  listen 443 ssl;
  server_name umbra.example.internal;

  ssl_certificate           /etc/nginx/certs/gateway-cert.pem;
  ssl_certificate_key       /etc/nginx/certs/gateway-key.pem;
  ssl_client_certificate    /etc/nginx/certs/client-ca-chain.pem;
  ssl_verify_client         on;
  ssl_verify_depth          2;

  if ($ssl_client_verify != SUCCESS) { return 401; }

  # Example extraction placeholders from client subject DN.
  # Replace with org-approved extraction/validation implementation.
  set $umbra_user   "";
  set $umbra_roles  "";
  set $umbra_tenant "";

  if ($ssl_client_s_dn ~ "CN=([^,]+)") { set $umbra_user $1; }
  if ($ssl_client_s_dn ~ "OU=([^,]+)") { set $umbra_roles $1; }
  if ($ssl_client_s_dn ~ "O=([0-9a-fA-F-]{36})") { set $umbra_tenant $1; }

  location / {
    proxy_set_header x-umbra-user      $umbra_user;
    proxy_set_header x-umbra-roles     $umbra_roles;
    proxy_set_header x-umbra-tenant-id $umbra_tenant;

    proxy_set_header x-umbra-request-id $http_x_umbra_request_id;
    proxy_set_header traceparent        $http_traceparent;

    proxy_set_header host $host;
    proxy_pass http://umbra-ui-upstream;
  }

  location /api/ {
    proxy_set_header x-umbra-user      $umbra_user;
    proxy_set_header x-umbra-roles     $umbra_roles;
    proxy_set_header x-umbra-tenant-id $umbra_tenant;

    proxy_set_header x-umbra-request-id $http_x_umbra_request_id;
    proxy_set_header traceparent        $http_traceparent;

    proxy_set_header host $host;
    proxy_pass http://umbra-controlplane-api-upstream;
  }
}
```

## Local demo path (without full mTLS)
For local demos, you may front Umbra with a non-mTLS proxy that injects static test headers.

Constraints:
- Local/demo only.
- Must not be treated as equivalent to production mTLS assurance.
- Should be clearly labeled in demo notes as a simulation path.

