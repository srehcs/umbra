package httpapi

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	roleAdmin         = "admin"
	roleToolAdmin     = "tool_admin"
	roleToolReader    = "tool_reader"
	rolePolicyAdmin   = "policy_admin"
	rolePolicyReader  = "policy_reader"
	roleAuditor       = "auditor"
	roleReceiptWriter = "receipt_writer"
)

var errUnauthorized = errors.New("unauthorized")

type authPrincipal struct {
	UserID   string
	TenantID uuid.UUID
	Roles    []string
}

func (p authPrincipal) HasAnyRole(wanted ...string) bool {
	if len(wanted) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(p.Roles))
	for _, role := range p.Roles {
		role = normalizeRole(role)
		if role != "" {
			set[role] = struct{}{}
		}
	}
	for _, role := range wanted {
		if _, ok := set[normalizeRole(role)]; ok {
			return true
		}
	}
	return false
}

type authConfig struct {
	Enabled     bool
	Issuer      string
	Audience    string
	HS256Secret []byte
	JWKSURL     string

	httpClient *http.Client

	mu         sync.RWMutex
	cachedJWKS map[string]*rsa.PublicKey
	cachedURL  string
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid,omitempty"`
}

type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Alg string `json:"alg"`
	E   string `json:"e"`
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	Use string `json:"use"`
}

func newAuthConfigFromEnv() (*authConfig, error) {
	cfg := &authConfig{
		Enabled:  envEnabled("UMBRA_AUTH_ENABLED"),
		Issuer:   firstNonEmptyEnv("OIDC_ISSUER_URL", "UMBRA_AUTH_JWT_ISSUER"),
		Audience: strings.TrimSpace(os.Getenv("UMBRA_AUTH_JWT_AUDIENCE")),
		JWKSURL:  strings.TrimSpace(os.Getenv("OIDC_JWKS_URL")),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	if secret := strings.TrimSpace(os.Getenv("UMBRA_AUTH_JWT_HS256_SECRET")); secret != "" {
		cfg.HS256Secret = []byte(secret)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (cfg *authConfig) authenticate(r *http.Request) (authPrincipal, error) {
	if cfg == nil || !cfg.Enabled {
		return authPrincipal{}, errUnauthorized
	}
	if err := cfg.validate(); err != nil {
		return authPrincipal{}, fmt.Errorf("%w: %v", errUnauthorized, err)
	}
	token, err := bearerToken(r.Header.Get("authorization"))
	if err != nil {
		return authPrincipal{}, fmt.Errorf("%w: %v", errUnauthorized, err)
	}
	claims, err := cfg.parseAndValidateJWT(token, time.Now().UTC())
	if err != nil {
		return authPrincipal{}, fmt.Errorf("%w: %v", errUnauthorized, err)
	}
	principal, err := principalFromClaims(claims)
	if err != nil {
		return authPrincipal{}, fmt.Errorf("%w: %v", errUnauthorized, err)
	}
	return principal, nil
}

func bearerToken(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("authorization must be bearer token")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("missing bearer token")
	}
	return token, nil
}

func (cfg *authConfig) validate() error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if cfg.Audience == "" {
		return errors.New("auth requires UMBRA_AUTH_JWT_AUDIENCE when UMBRA_AUTH_ENABLED=true")
	}
	if len(cfg.HS256Secret) == 0 && cfg.Issuer == "" && cfg.JWKSURL == "" {
		return errors.New("auth requires UMBRA_AUTH_JWT_HS256_SECRET or OIDC_ISSUER_URL/OIDC_JWKS_URL when UMBRA_AUTH_ENABLED=true")
	}
	return nil
}

func (cfg *authConfig) parseAndValidateJWT(token string, now time.Time) (map[string]interface{}, error) {
	parts, header, err := parseJWTHeader(token)
	if err != nil {
		return nil, err
	}

	switch strings.TrimSpace(header.Alg) {
	case "HS256":
		if len(cfg.HS256Secret) == 0 {
			return nil, errors.New("hs256 secret unavailable")
		}
		if err := validateHS256JWT(parts, cfg.HS256Secret); err != nil {
			return nil, err
		}
	case "RS256":
		publicKey, err := cfg.lookupRSAPublicKey(header.Kid)
		if err != nil {
			return nil, err
		}
		if err := validateRS256JWT(parts, publicKey); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unsupported jwt alg")
	}

	claims, err := decodeJWTClaims(parts[1])
	if err != nil {
		return nil, err
	}
	if err := validateJWTClaims(claims, cfg.Issuer, cfg.Audience, now); err != nil {
		return nil, err
	}
	return claims, nil
}

func parseJWTHeader(token string) ([3]string, jwtHeader, error) {
	var zero [3]string
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return zero, jwtHeader{}, errors.New("invalid jwt format")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return zero, jwtHeader{}, errors.New("invalid jwt header encoding")
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return zero, jwtHeader{}, errors.New("invalid jwt header")
	}
	return [3]string{parts[0], parts[1], parts[2]}, header, nil
}

func validateHS256JWT(parts [3]string, secret []byte) error {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	mac.Write([]byte("."))
	mac.Write([]byte(parts[1]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return errors.New("invalid jwt signature encoding")
	}
	if !hmac.Equal(got, expected) {
		return errors.New("invalid jwt signature")
	}
	return nil
}

func validateRS256JWT(parts [3]string, publicKey *rsa.PublicKey) error {
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return errors.New("invalid jwt signature encoding")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return errors.New("invalid jwt signature")
	}
	return nil
}

func decodeJWTClaims(claimPart string) (map[string]interface{}, error) {
	claimBytes, err := base64.RawURLEncoding.DecodeString(claimPart)
	if err != nil {
		return nil, errors.New("invalid jwt claims encoding")
	}
	claims := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(string(claimBytes)))
	dec.UseNumber()
	if err := dec.Decode(&claims); err != nil {
		return nil, errors.New("invalid jwt claims")
	}
	return claims, nil
}

func (cfg *authConfig) lookupRSAPublicKey(kid string) (*rsa.PublicKey, error) {
	kid = strings.TrimSpace(kid)
	if kid == "" {
		return nil, errors.New("missing jwt kid")
	}

	cfg.mu.RLock()
	if key, ok := cfg.cachedJWKS[kid]; ok {
		cfg.mu.RUnlock()
		return key, nil
	}
	cfg.mu.RUnlock()

	jwksURL, err := cfg.resolveJWKSURL()
	if err != nil {
		return nil, err
	}
	if err := cfg.refreshJWKS(jwksURL); err != nil {
		return nil, err
	}

	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	key, ok := cfg.cachedJWKS[kid]
	if !ok {
		return nil, errors.New("jwk not found")
	}
	return key, nil
}

func (cfg *authConfig) resolveJWKSURL() (string, error) {
	if cfg.JWKSURL != "" {
		return cfg.JWKSURL, nil
	}
	if cfg.Issuer == "" {
		return "", errors.New("jwks configuration unavailable")
	}
	discoveryURL := strings.TrimRight(cfg.Issuer, "/") + "/.well-known/openid-configuration"
	resp, err := cfg.httpClient.Get(discoveryURL)
	if err != nil {
		return "", errors.New("oidc discovery failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("oidc discovery failed")
	}
	var discovery oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return "", errors.New("oidc discovery invalid")
	}
	if strings.TrimSpace(discovery.JWKSURI) == "" {
		return "", errors.New("oidc discovery invalid")
	}
	return discovery.JWKSURI, nil
}

func (cfg *authConfig) refreshJWKS(jwksURL string) error {
	resp, err := cfg.httpClient.Get(jwksURL)
	if err != nil {
		return errors.New("jwks fetch failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("jwks fetch failed")
	}
	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return errors.New("jwks invalid")
	}
	keys := make(map[string]*rsa.PublicKey)
	for _, item := range doc.Keys {
		if item.Kty != "RSA" || item.Kid == "" || item.N == "" || item.E == "" {
			continue
		}
		publicKey, err := jwkToRSAPublicKey(item)
		if err != nil {
			continue
		}
		keys[item.Kid] = publicKey
	}
	if len(keys) == 0 {
		return errors.New("jwks invalid")
	}
	cfg.mu.Lock()
	cfg.cachedURL = jwksURL
	cfg.cachedJWKS = keys
	cfg.mu.Unlock()
	return nil
}

func jwkToRSAPublicKey(key jwk) (*rsa.PublicKey, error) {
	modulusBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, err
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, err
	}
	modulus := new(big.Int).SetBytes(modulusBytes)
	exponent := new(big.Int).SetBytes(exponentBytes)
	if !exponent.IsInt64() || exponent.Int64() <= 0 {
		return nil, errors.New("invalid jwk exponent")
	}
	return &rsa.PublicKey{
		N: modulus,
		E: int(exponent.Int64()),
	}, nil
}

func validateJWTClaims(claims map[string]interface{}, wantIssuer, wantAudience string, now time.Time) error {
	if wantIssuer != "" {
		if strings.TrimSpace(claimString(claims["iss"])) != wantIssuer {
			return errors.New("unexpected jwt issuer")
		}
	}
	if wantAudience != "" && !claimAudienceContains(claims["aud"], wantAudience) {
		return errors.New("unexpected jwt audience")
	}
	nowUnix := now.Unix()
	if exp, ok := claimInt64(claims["exp"]); ok && nowUnix >= exp {
		return errors.New("jwt expired")
	}
	if nbf, ok := claimInt64(claims["nbf"]); ok && nowUnix < nbf {
		return errors.New("jwt not yet valid")
	}
	return nil
}

func principalFromClaims(claims map[string]interface{}) (authPrincipal, error) {
	userID := strings.TrimSpace(claimString(claims["sub"]))
	if userID == "" {
		userID = strings.TrimSpace(claimString(claims["preferred_username"]))
	}
	if userID == "" {
		return authPrincipal{}, errors.New("jwt subject missing")
	}
	tenantRaw := strings.TrimSpace(claimString(claims["tenant_id"]))
	if tenantRaw == "" {
		tenantRaw = strings.TrimSpace(claimString(claims["x-umbra-tenant-id"]))
	}
	if tenantRaw == "" {
		return authPrincipal{}, errors.New("tenant claim missing")
	}
	tenantID, err := uuid.Parse(tenantRaw)
	if err != nil {
		return authPrincipal{}, errors.New("invalid tenant claim")
	}
	roles := collectClaimRoles(claims)
	if len(roles) == 0 {
		return authPrincipal{}, errors.New("roles claim missing")
	}
	return authPrincipal{
		UserID:   userID,
		TenantID: tenantID,
		Roles:    roles,
	}, nil
}

func collectClaimRoles(claims map[string]interface{}) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(raw interface{}, normalize func(string) string) {
		for _, role := range toRoleList(raw) {
			role = normalize(role)
			if role == "" {
				continue
			}
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			out = append(out, role)
		}
	}

	add(claims["roles"], normalizeRole)
	add(claims["groups"], normalizeGroupRole)
	if realmRaw, ok := claims["realm_access"]; ok {
		if realm, ok := realmRaw.(map[string]interface{}); ok {
			add(realm["roles"], normalizeRole)
		}
	}
	if resourceRaw, ok := claims["resource_access"]; ok {
		if resources, ok := resourceRaw.(map[string]interface{}); ok {
			if umbraRaw, ok := resources["umbra"]; ok {
				if umbra, ok := umbraRaw.(map[string]interface{}); ok {
					add(umbra["roles"], normalizeRole)
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

func toRoleList(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		return parseCSV(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func claimString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func claimInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case float64:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func claimAudienceContains(raw interface{}, want string) bool {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v) == want
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) == want {
				return true
			}
		}
	}
	return false
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func firstNonEmptyEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeRole(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	return raw
}

func normalizeGroupRole(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 && parts[0] == "umbra" {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
