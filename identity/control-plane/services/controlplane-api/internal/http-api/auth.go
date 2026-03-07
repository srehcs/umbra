package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	roleAdmin        = "admin"
	roleToolAdmin    = "tool_admin"
	roleToolReader   = "tool_reader"
	rolePolicyAdmin  = "policy_admin"
	rolePolicyReader = "policy_reader"
	roleAuditor      = "auditor"
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
		role = strings.ToLower(strings.TrimSpace(role))
		if role != "" {
			set[role] = struct{}{}
		}
	}
	for _, role := range wanted {
		if _, ok := set[strings.ToLower(strings.TrimSpace(role))]; ok {
			return true
		}
	}
	return false
}

type authConfig struct {
	Enabled    bool
	Issuer     string
	Audience   string
	HS256Secret []byte
}

func newAuthConfigFromEnv() (*authConfig, error) {
	cfg := &authConfig{
		Enabled:  envEnabled("UMBRA_AUTH_ENABLED"),
		Issuer:   strings.TrimSpace(os.Getenv("UMBRA_AUTH_JWT_ISSUER")),
		Audience: strings.TrimSpace(os.Getenv("UMBRA_AUTH_JWT_AUDIENCE")),
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	secret := os.Getenv("UMBRA_AUTH_JWT_HS256_SECRET")
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("UMBRA_AUTH_JWT_HS256_SECRET required when UMBRA_AUTH_ENABLED=true")
	}
	cfg.HS256Secret = []byte(secret)
	return cfg, nil
}

func (cfg *authConfig) authenticate(r *http.Request) (authPrincipal, error) {
	if cfg == nil || !cfg.Enabled {
		return authPrincipal{}, errUnauthorized
	}
	token, err := bearerToken(r.Header.Get("authorization"))
	if err != nil {
		return authPrincipal{}, fmt.Errorf("%w: %v", errUnauthorized, err)
	}
	claims, err := parseAndValidateHS256JWT(token, cfg.HS256Secret, cfg.Issuer, cfg.Audience, time.Now().UTC())
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

func parseAndValidateHS256JWT(token string, secret []byte, wantIssuer, wantAudience string, now time.Time) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid jwt header encoding")
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid jwt header")
	}
	if strings.TrimSpace(header.Alg) != "HS256" {
		return nil, errors.New("unsupported jwt alg")
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	mac.Write([]byte("."))
	mac.Write([]byte(parts[1]))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid jwt signature encoding")
	}
	if !hmac.Equal(got, expected) {
		return nil, errors.New("invalid jwt signature")
	}

	claimBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid jwt claims encoding")
	}
	claims := map[string]interface{}{}
	dec := json.NewDecoder(strings.NewReader(string(claimBytes)))
	dec.UseNumber()
	if err := dec.Decode(&claims); err != nil {
		return nil, errors.New("invalid jwt claims")
	}
	if err := validateJWTClaims(claims, wantIssuer, wantAudience, now); err != nil {
		return nil, err
	}
	return claims, nil
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
	add := func(raw interface{}) {
		for _, role := range toRoleList(raw) {
			role = strings.ToLower(strings.TrimSpace(role))
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

	add(claims["roles"])
	if realmRaw, ok := claims["realm_access"]; ok {
		if realm, ok := realmRaw.(map[string]interface{}); ok {
			add(realm["roles"])
		}
	}
	if resourceRaw, ok := claims["resource_access"]; ok {
		if resources, ok := resourceRaw.(map[string]interface{}); ok {
			if umbraRaw, ok := resources["umbra"]; ok {
				if umbra, ok := umbraRaw.(map[string]interface{}); ok {
					add(umbra["roles"])
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
