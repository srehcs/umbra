import { randomBytes } from 'node:crypto';
import type { ResponseCookie } from 'next/dist/compiled/@edge-runtime/cookies';
import { cookies, headers } from 'next/headers';

import { authenticateBearerToken, authenticateToken } from './jwt';
import { getServerAuthConfig, getServerOIDCConfig } from './settings';

export const ACCESS_TOKEN_COOKIE = 'umbra_access_token';
export const OIDC_STATE_COOKIE = 'umbra_oidc_state';
export const OIDC_VERIFIER_COOKIE = 'umbra_oidc_verifier';

type CookieOptions = Pick<
  ResponseCookie,
  'httpOnly' | 'maxAge' | 'path' | 'sameSite' | 'secure'
>;

type OIDCDiscovery = {
  authorization_endpoint: string;
  end_session_endpoint?: string | undefined;
  issuer: string;
  token_endpoint: string;
};

const discoveryCache = new Map<string, Promise<OIDCDiscovery>>();

export function readAccessTokenFromCookies(): string | undefined {
  return cookies().get(ACCESS_TOKEN_COOKIE)?.value?.trim() || undefined;
}

export function clearAccessTokenCookie() {
  cookies().set(ACCESS_TOKEN_COOKIE, '', expiredCookieOptions());
}

export async function authenticateRequest(): Promise<
  Awaited<ReturnType<typeof authenticateToken>>
> {
  const headerToken = authorizationHeaderFromRequest();
  if (headerToken) {
    return authenticateBearerToken(headerToken, getServerAuthConfig());
  }

  const cookieToken = readAccessTokenFromCookies();
  if (!cookieToken) {
    throw new Error('missing access token');
  }
  return authenticateToken(cookieToken, getServerAuthConfig());
}

export async function setAccessTokenCookie(token: string) {
  const principal = await authenticateToken(token, getServerAuthConfig());
  const maxAge = principal.expiresAt
    ? Math.max(principal.expiresAt - Math.floor(Date.now() / 1000), 0)
    : 3600;
  cookies().set(ACCESS_TOKEN_COOKIE, token, baseCookieOptions(maxAge));
  return principal;
}

export async function createOIDCRedirectURL() {
  const discovery = await getOIDCDiscovery();
  const oidc = getServerOIDCConfig();
  const state = randomBase64URL(24);
  const verifier = randomBase64URL(48);
  const challenge = await pkceChallenge(verifier);

  cookies().set(OIDC_STATE_COOKIE, state, transientCookieOptions());
  cookies().set(OIDC_VERIFIER_COOKIE, verifier, transientCookieOptions());

  const url = new URL(discovery.authorization_endpoint);
  url.searchParams.set('client_id', oidc.clientId);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('scope', oidc.scope);
  url.searchParams.set('redirect_uri', oidc.redirectURI);
  url.searchParams.set('state', state);
  url.searchParams.set('code_challenge', challenge);
  url.searchParams.set('code_challenge_method', 'S256');
  return url.toString();
}

export async function exchangeOIDCCode(
  code: string,
  returnedState: string | null,
): Promise<Awaited<ReturnType<typeof authenticateToken>>> {
  const oidc = getServerOIDCConfig();
  const state = cookies().get(OIDC_STATE_COOKIE)?.value ?? '';
  const verifier = cookies().get(OIDC_VERIFIER_COOKIE)?.value ?? '';
  clearTransientOIDCCookies();
  if (
    !code ||
    !returnedState ||
    !state ||
    returnedState !== state ||
    !verifier
  ) {
    throw new Error('invalid oidc callback');
  }

  const discovery = await getOIDCDiscovery();
  const body = new URLSearchParams({
    client_id: oidc.clientId,
    code,
    code_verifier: verifier,
    grant_type: 'authorization_code',
    redirect_uri: oidc.redirectURI,
  });
  if (oidc.clientSecret) {
    body.set('client_secret', oidc.clientSecret);
  }

  const response = await fetch(discovery.token_endpoint, {
    body,
    cache: 'no-store',
    headers: {
      'content-type': 'application/x-www-form-urlencoded',
    },
    method: 'POST',
  });
  if (!response.ok) {
    throw new Error('oidc token exchange failed');
  }

  const payload = (await response.json()) as {
    access_token?: string;
  };
  const accessToken = payload.access_token?.trim();
  if (!accessToken) {
    throw new Error('oidc access token missing');
  }
  return setAccessTokenCookie(accessToken);
}

export async function getOIDCLogoutURL() {
  const discovery = await getOIDCDiscovery();
  const postLogoutRedirect = getServerOIDCConfig().postLogoutRedirectURI;
  if (!discovery.end_session_endpoint) {
    return postLogoutRedirect;
  }
  const url = new URL(discovery.end_session_endpoint);
  url.searchParams.set('post_logout_redirect_uri', postLogoutRedirect);
  return url.toString();
}

function authorizationHeaderFromRequest(): string | null {
  return headers().get('authorization');
}

async function getOIDCDiscovery(): Promise<OIDCDiscovery> {
  const { issuer } = getServerOIDCConfig();
  const cached = discoveryCache.get(issuer);
  if (cached) {
    return cached;
  }
  const promise = (async () => {
    const url = new URL(
      '/.well-known/openid-configuration',
      ensureTrailingSlash(issuer),
    );
    const response = await fetch(url, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error('oidc discovery failed');
    }
    const body = (await response.json()) as Partial<OIDCDiscovery>;
    if (!body.authorization_endpoint || !body.token_endpoint || !body.issuer) {
      throw new Error('oidc discovery invalid');
    }
    return {
      authorization_endpoint: body.authorization_endpoint,
      end_session_endpoint: body.end_session_endpoint,
      issuer: body.issuer,
      token_endpoint: body.token_endpoint,
    };
  })();
  discoveryCache.set(issuer, promise);
  return promise;
}

function ensureTrailingSlash(value: string): string {
  return value.endsWith('/') ? value : `${value}/`;
}

function clearTransientOIDCCookies() {
  cookies().set(OIDC_STATE_COOKIE, '', expiredCookieOptions());
  cookies().set(OIDC_VERIFIER_COOKIE, '', expiredCookieOptions());
}

function baseCookieOptions(maxAge: number): CookieOptions {
  return {
    httpOnly: true,
    maxAge,
    path: '/',
    sameSite: 'lax',
    secure: process.env.NODE_ENV === 'production',
  };
}

function transientCookieOptions(): CookieOptions {
  return baseCookieOptions(10 * 60);
}

function expiredCookieOptions(): CookieOptions {
  return {
    ...baseCookieOptions(0),
    maxAge: 0,
  };
}

function randomBase64URL(bytes: number): string {
  return randomBytes(bytes).toString('base64url');
}

async function pkceChallenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(verifier),
  );
  return Buffer.from(digest).toString('base64url');
}
