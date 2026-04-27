package httpapi

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

func TestNewAuthConfigFromEnvRequiresAudienceWhenEnabled(t *testing.T) {
	t.Setenv("UMBRA_AUTH_ENABLED", "true")
	t.Setenv("UMBRA_AUTH_JWT_AUDIENCE", "")
	t.Setenv("UMBRA_AUTH_JWT_HS256_SECRET", "test-secret")
	_, err := newAuthConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "UMBRA_AUTH_JWT_AUDIENCE") {
		t.Fatalf("expected auth config audience error, got %v", err)
	}
}

func TestNewAuthConfigFromEnvRequiresVerifierConfigWhenEnabled(t *testing.T) {
	t.Setenv("UMBRA_AUTH_ENABLED", "true")
	t.Setenv("UMBRA_AUTH_JWT_AUDIENCE", "umbra-controlplane")
	t.Setenv("UMBRA_AUTH_JWT_HS256_SECRET", "")
	_, err := newAuthConfigFromEnv()
	if err == nil {
		t.Fatal("expected auth config error when verifier config missing")
	}
}

func TestHandlePoliciesAuthEnabledRejectsMissingToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:     true,
			Audience:    "umbra-controlplane",
			HS256Secret: []byte("test-secret"),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var out protocol.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if out.Error.Code != protocol.ErrorCodeUnauthorized {
		t.Fatalf("expected %s, got %s", protocol.ErrorCodeUnauthorized, out.Error.Code)
	}
}

func TestHandlePoliciesAuthEnabledRejectsMissingAudienceConfig(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:     true,
			HS256Secret: []byte("test-secret"),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("authorization", "Bearer ignored")
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleReceiptsListAuthEnabledRejectsInsufficientRole(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	secret := []byte("test-secret")
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:     true,
			Audience:    "umbra-controlplane",
			HS256Secret: secret,
		},
	}

	token := mustSignHS256Token(t, secret, map[string]interface{}{
		"sub":       "user-1",
		"aud":       "umbra-controlplane",
		"tenant_id": uuid.NewString(),
		"roles":     []string{rolePolicyReader},
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/receipts", nil)
	req.Header.Set("authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.handleReceipts(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out protocol.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if out.Error.Code != protocol.ErrorCodeForbidden {
		t.Fatalf("expected %s, got %s", protocol.ErrorCodeForbidden, out.Error.Code)
	}
}

func TestHandlePoliciesAuthEnabledDerivesTenantFromClaims(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	secret := []byte("test-secret")
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:     true,
			HS256Secret: secret,
			Issuer:      "https://issuer.example",
			Audience:    "umbra-controlplane",
		},
	}
	tenantID := uuid.NewString()
	token := mustSignHS256Token(t, secret, map[string]interface{}{
		"sub":       "user-1",
		"iss":       "https://issuer.example",
		"aud":       "umbra-controlplane",
		"tenant_id": tenantID,
		"roles":     []string{rolePolicyReader},
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)

	// Auth should succeed, then fail on missing store.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (auth passed), got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := req.Header.Get("x-umbra-tenant-id"); got != tenantID {
		t.Fatalf("expected tenant header derived from claims, got %q", got)
	}
	if got := req.Header.Get("x-umbra-user"); got != "user-1" {
		t.Fatalf("expected user header from claims, got %q", got)
	}
}

func TestHandlePoliciesAuthEnabledIgnoresSpoofedTenantHeader(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	secret := []byte("test-secret")
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:     true,
			Audience:    "umbra-controlplane",
			HS256Secret: secret,
		},
	}

	claimedTenant := uuid.NewString()
	token := mustSignHS256Token(t, secret, map[string]interface{}{
		"sub":       "user-1",
		"aud":       "umbra-controlplane",
		"tenant_id": claimedTenant,
		"roles":     []string{rolePolicyReader},
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (auth passed), got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := req.Header.Get("x-umbra-tenant-id"); got != claimedTenant {
		t.Fatalf("expected claim tenant %q to override header, got %q", claimedTenant, got)
	}
}

func TestPrincipalFromClaimsSupportsGroupsClaim(t *testing.T) {
	tenantID := uuid.NewString()
	principal, err := principalFromClaims(map[string]interface{}{
		"sub":       "user-1",
		"tenant_id": tenantID,
		"groups":    []interface{}{"/umbra/policy_admin", "/umbra/auditor"},
	})
	if err != nil {
		t.Fatalf("principalFromClaims returned error: %v", err)
	}
	if principal.TenantID.String() != tenantID {
		t.Fatalf("expected tenant %q, got %q", tenantID, principal.TenantID.String())
	}
	if !principal.HasAnyRole(rolePolicyAdmin) {
		t.Fatalf("expected policy admin role from groups claim")
	}
	if !principal.HasAnyRole(roleAuditor) {
		t.Fatalf("expected auditor role from groups claim")
	}
}

func TestPrincipalFromClaimsRejectsUnscopedGroupPaths(t *testing.T) {
	_, err := principalFromClaims(map[string]interface{}{
		"sub":       "user-1",
		"tenant_id": uuid.NewString(),
		"groups":    []interface{}{"/finance/policy_admin"},
	})
	if err == nil || !strings.Contains(err.Error(), "roles claim missing") {
		t.Fatalf("expected unscoped group path to be rejected, got %v", err)
	}
}

func TestHandlePoliciesAuthEnabledAcceptsRS256TokenViaOIDCDiscovery(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	var issuer string
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":   issuer,
				"jwks_uri": issuer + "/jwks",
			})
		case "/jwks":
			pub := privateKey.PublicKey
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"keys": []map[string]string{{
					"kid": "test-kid",
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer jwksServer.Close()
	issuer = jwksServer.URL

	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth: &authConfig{
			Enabled:    true,
			Issuer:     issuer,
			Audience:   "umbra-controlplane",
			httpClient: jwksServer.Client(),
		},
	}
	tenantID := uuid.NewString()
	token := mustSignRS256Token(t, privateKey, "test-kid", map[string]interface{}{
		"sub":       "user-1",
		"iss":       issuer,
		"aud":       "umbra-controlplane",
		"tenant_id": tenantID,
		"roles":     []string{rolePolicyReader},
		"exp":       time.Now().Add(5 * time.Minute).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (auth passed), got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := req.Header.Get("x-umbra-tenant-id"); got != tenantID {
		t.Fatalf("expected tenant header derived from claims, got %q", got)
	}
}

func TestHandlePoliciesAuthDisabledUsesTenantHeader(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := &Server{Logger: logger, Store: nil, Auth: &authConfig{Enabled: false}}
	req := httptest.NewRequest(http.MethodGet, "/v1/policies", nil)
	req.Header.Set("x-umbra-tenant-id", uuid.NewString())
	rec := httptest.NewRecorder()
	server.handlePolicies(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when auth disabled and tenant header present, got %d", rec.Code)
	}
}

func mustSignHS256Token(t *testing.T, secret []byte, claims map[string]interface{}) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	headerPart := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsPart := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signed := headerPart + "." + claimsPart

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signed))
	signaturePart := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signed + "." + signaturePart
}

func mustSignRS256Token(t *testing.T, privateKey *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": kid,
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	headerPart := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsPart := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signed := headerPart + "." + claimsPart

	sum := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	signaturePart := base64.RawURLEncoding.EncodeToString(signature)
	return signed + "." + signaturePart
}
