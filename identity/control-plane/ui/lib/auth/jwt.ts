import {
  createHmac,
  createPublicKey,
  createVerify,
  timingSafeEqual,
} from 'node:crypto';

type JWTClaims = Record<string, unknown>;

type JWK = {
  alg?: string;
  e?: string;
  kid?: string;
  kty?: string;
  n?: string;
  use?: string;
};

type JWKSResponse = {
  keys?: JWK[];
};

export type JWTConfig = {
  audience?: string | undefined;
  hs256Secret?: string | undefined;
  issuer?: string | undefined;
  jwksURL?: string | undefined;
};

export type SessionPrincipal = {
  userId: string;
  tenantId: string;
  roles: string[];
  expiresAt?: number | undefined;
};

type DiscoveryDocument = {
  issuer: string;
  jwks_uri: string;
};

const discoveryCache = new Map<string, Promise<DiscoveryDocument>>();
const jwksCache = new Map<string, Promise<JWKSResponse>>();

export async function authenticateBearerToken(
  authorizationHeader: string | null,
  config: JWTConfig,
): Promise<SessionPrincipal> {
  const token = bearerToken(authorizationHeader);
  return authenticateToken(token, config);
}

export async function authenticateToken(
  token: string,
  config: JWTConfig,
): Promise<SessionPrincipal> {
  const claims = await parseAndValidateJWT(token, config);
  return principalFromClaims(claims);
}

export function bearerToken(raw: string | null): string {
  const value = raw?.trim() ?? '';
  if (!value) {
    throw new Error('missing authorization header');
  }
  const [scheme = '', tokenPart = ''] = value.split(/\s+/, 2);
  if (!scheme || !tokenPart || scheme.toLowerCase() !== 'bearer') {
    throw new Error('authorization must be bearer token');
  }
  const token = tokenPart.trim();
  if (!token) {
    throw new Error('missing bearer token');
  }
  return token;
}

export function decodeClaimsWithoutVerification(token: string): JWTClaims {
  const parts = token.split('.');
  if (parts.length !== 3) {
    throw new Error('invalid jwt format');
  }
  return decodeJSONPart(parts[1] ?? '') as JWTClaims;
}

async function parseAndValidateJWT(
  token: string,
  config: JWTConfig,
): Promise<JWTClaims> {
  const parts = token.split('.');
  if (parts.length !== 3) {
    throw new Error('invalid jwt format');
  }

  const [headerPart = '', claimsPart = '', signaturePart = ''] = parts;
  const header = decodeJSONPart(headerPart);
  const alg = typeof header.alg === 'string' ? header.alg.trim() : '';
  const payload = `${headerPart}.${claimsPart}`;

  switch (alg) {
    case 'HS256':
      validateHS256Signature(payload, signaturePart, config);
      break;
    case 'RS256':
      await validateRS256Signature(payload, signaturePart, header, config);
      break;
    default:
      throw new Error('unsupported jwt alg');
  }

  const claims = decodeJSONPart(claimsPart) as JWTClaims;
  validateJWTClaims(claims, config);
  return claims;
}

function validateHS256Signature(
  payload: string,
  signaturePart: string,
  config: JWTConfig,
) {
  if (!config.hs256Secret) {
    throw new Error('hs256 secret unavailable');
  }
  const expected = createHmac('sha256', config.hs256Secret)
    .update(payload)
    .digest();
  const actual = decodeBase64URL(signaturePart);
  if (actual.length !== expected.length || !timingSafeEqual(actual, expected)) {
    throw new Error('invalid jwt signature');
  }
}

async function validateRS256Signature(
  payload: string,
  signaturePart: string,
  header: Record<string, unknown>,
  config: JWTConfig,
) {
  const kid = typeof header.kid === 'string' ? header.kid.trim() : '';
  if (!kid) {
    throw new Error('missing jwt kid');
  }
  const jwk = await getJWK(config, kid);
  const verifier = createVerify('RSA-SHA256');
  verifier.update(payload);
  verifier.end();
  const signature = decodeBase64URL(signaturePart);
  const publicKey = createPublicKey({
    format: 'jwk',
    key: {
      alg: 'RS256',
      e: jwk.e,
      key_ops: ['verify'],
      kid: jwk.kid,
      kty: jwk.kty,
      n: jwk.n,
      use: 'sig',
    },
  });
  if (!verifier.verify(publicKey, signature)) {
    throw new Error('invalid jwt signature');
  }
}

async function getJWK(config: JWTConfig, kid: string): Promise<JWK> {
  const jwksURL = await resolveJWKSURL(config);
  const jwks = await getJWKS(jwksURL);
  const key = (jwks.keys ?? []).find(
    (candidate) =>
      candidate.kid === kid &&
      candidate.kty === 'RSA' &&
      candidate.n &&
      candidate.e,
  );
  if (!key) {
    jwksCache.delete(jwksURL);
    const reloaded = await getJWKS(jwksURL);
    const retry = (reloaded.keys ?? []).find(
      (candidate) =>
        candidate.kid === kid &&
        candidate.kty === 'RSA' &&
        candidate.n &&
        candidate.e,
    );
    if (!retry) {
      throw new Error('jwk not found');
    }
    return retry;
  }
  return key;
}

async function resolveJWKSURL(config: JWTConfig): Promise<string> {
  if (config.jwksURL) {
    return config.jwksURL;
  }
  if (!config.issuer) {
    throw new Error('jwks configuration unavailable');
  }
  const discovery = await getDiscoveryDocument(config.issuer);
  return discovery.jwks_uri;
}

async function getDiscoveryDocument(
  issuer: string,
): Promise<DiscoveryDocument> {
  const cached = discoveryCache.get(issuer);
  if (cached) {
    return cached;
  }
  const promise = (async () => {
    const discoveryURL = new URL(
      '/.well-known/openid-configuration',
      ensureTrailingSlash(issuer),
    );
    const response = await fetch(discoveryURL, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error('oidc discovery failed');
    }
    const body = (await response.json()) as Partial<DiscoveryDocument>;
    if (!body.issuer || !body.jwks_uri) {
      throw new Error('oidc discovery invalid');
    }
    return {
      issuer: body.issuer,
      jwks_uri: body.jwks_uri,
    };
  })();
  discoveryCache.set(issuer, promise);
  return promise;
}

async function getJWKS(jwksURL: string): Promise<JWKSResponse> {
  const cached = jwksCache.get(jwksURL);
  if (cached) {
    return cached;
  }
  const promise = (async () => {
    const response = await fetch(jwksURL, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error('jwks fetch failed');
    }
    const body = (await response.json()) as JWKSResponse;
    if (!Array.isArray(body.keys)) {
      throw new Error('jwks invalid');
    }
    return body;
  })();
  jwksCache.set(jwksURL, promise);
  return promise;
}

function ensureTrailingSlash(value: string): string {
  return value.endsWith('/') ? value : `${value}/`;
}

function decodeJSONPart(part: string): Record<string, unknown> {
  const decoded = decodeBase64URL(part).toString('utf8');
  const parsed = JSON.parse(decoded);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('invalid jwt json');
  }
  return parsed as Record<string, unknown>;
}

function decodeBase64URL(part: string): Buffer {
  return Buffer.from(part, 'base64url');
}

function validateJWTClaims(claims: JWTClaims, config: JWTConfig) {
  if (config.issuer && claimString(claims.iss) !== config.issuer) {
    throw new Error('unexpected jwt issuer');
  }
  if (config.audience && !claimAudienceContains(claims.aud, config.audience)) {
    throw new Error('unexpected jwt audience');
  }

  const now = Math.floor(Date.now() / 1000);
  const exp = claimNumber(claims.exp);
  if (exp !== undefined && now >= exp) {
    throw new Error('jwt expired');
  }
  const nbf = claimNumber(claims.nbf);
  if (nbf !== undefined && now < nbf) {
    throw new Error('jwt not yet valid');
  }
}

function principalFromClaims(claims: JWTClaims): SessionPrincipal {
  const userId =
    claimString(claims.sub) || claimString(claims.preferred_username);
  if (!userId) {
    throw new Error('jwt subject missing');
  }

  const tenantId =
    claimString(claims.tenant_id) || claimString(claims['x-umbra-tenant-id']);
  if (!tenantId || !isUUID(tenantId)) {
    throw new Error('invalid tenant claim');
  }

  const roles = collectClaimRoles(claims);
  if (roles.length === 0) {
    throw new Error('roles claim missing');
  }

  const expiresAt = claimNumber(claims.exp);
  return { userId, tenantId, roles, ...(expiresAt ? { expiresAt } : {}) };
}

function collectClaimRoles(claims: JWTClaims): string[] {
  const seen = new Set<string>();
  const roles: string[] = [];
  const add = (raw: unknown, normalize: (value: string) => string) => {
    for (const role of toRoleList(raw)) {
      const normalized = normalize(role);
      if (!normalized || seen.has(normalized)) continue;
      seen.add(normalized);
      roles.push(normalized);
    }
  };

  add(claims.roles, normalizeRole);
  add(claims.groups, normalizeGroupRole);
  if (isRecord(claims.realm_access)) {
    add(claims.realm_access.roles, normalizeRole);
  }
  if (
    isRecord(claims.resource_access) &&
    isRecord(claims.resource_access.umbra)
  ) {
    add(claims.resource_access.umbra.roles, normalizeRole);
  }
  return roles.sort();
}

function toRoleList(raw: unknown): string[] {
  if (typeof raw === 'string') {
    return raw
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean);
  }
  if (Array.isArray(raw)) {
    return raw.filter((value): value is string => typeof value === 'string');
  }
  return [];
}

function normalizeRole(raw: string): string {
  return raw.trim().toLowerCase();
}

function normalizeGroupRole(raw: string): string {
  const trimmed = raw.trim().toLowerCase();
  if (!trimmed) return '';
  const parts = trimmed.split('/').filter(Boolean);
  if (parts.length === 1) {
    return parts[0] ?? '';
  }
  if (parts.length === 2 && parts[0] === 'umbra') {
    return parts[1] ?? '';
  }
  return '';
}

function claimString(raw: unknown): string {
  return typeof raw === 'string' ? raw.trim() : '';
}

function claimNumber(raw: unknown): number | undefined {
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return Math.trunc(raw);
  }
  if (typeof raw === 'string' && raw.trim()) {
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function claimAudienceContains(raw: unknown, expected: string): boolean {
  if (typeof raw === 'string') {
    return raw.trim() === expected;
  }
  if (Array.isArray(raw)) {
    return raw.some(
      (value) => typeof value === 'string' && value.trim() === expected,
    );
  }
  return false;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isUUID(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    value,
  );
}
