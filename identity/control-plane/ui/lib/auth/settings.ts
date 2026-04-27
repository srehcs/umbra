import { isServerAuthEnabled } from './config';

export type ServerAuthConfig = {
  audience?: string | undefined;
  hs256Secret?: string | undefined;
  issuer?: string | undefined;
  jwksURL?: string | undefined;
};

export type ServerOIDCConfig = {
  clientId: string;
  clientSecret?: string | undefined;
  issuer: string;
  postLogoutRedirectURI: string;
  redirectURI: string;
  scope: string;
};

export function getServerAuthConfig(): ServerAuthConfig {
  if (!isServerAuthEnabled()) {
    return {};
  }
  return {
    audience: requiredEnv('UMBRA_AUTH_JWT_AUDIENCE'),
    hs256Secret: optionalEnv('UMBRA_AUTH_JWT_HS256_SECRET'),
    issuer:
      optionalEnv('OIDC_ISSUER_URL') ?? optionalEnv('UMBRA_AUTH_JWT_ISSUER'),
    jwksURL: optionalEnv('OIDC_JWKS_URL'),
  };
}

export function getServerOIDCConfig(): ServerOIDCConfig {
  const issuer = requiredEnv('OIDC_ISSUER_URL');
  const baseURL = optionalEnv('AUTH_BASE_URL') ?? 'http://localhost:3000';
  return {
    clientId: requiredEnv('OIDC_CLIENT_ID'),
    clientSecret: optionalEnv('OIDC_CLIENT_SECRET'),
    issuer,
    postLogoutRedirectURI: `${baseURL}/`,
    redirectURI: `${baseURL}/api/auth/callback`,
    scope: optionalEnv('OIDC_SCOPE') ?? 'openid profile email',
  };
}

function requiredEnv(name: string): string {
  const value = optionalEnv(name);
  if (!value) {
    throw new Error(`${name} required`);
  }
  return value;
}

function optionalEnv(name: string): string | undefined {
  const value = process.env[name]?.trim();
  return value ? value : undefined;
}
