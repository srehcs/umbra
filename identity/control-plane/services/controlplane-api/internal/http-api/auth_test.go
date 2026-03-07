package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

func TestNewAuthConfigFromEnvRequiresSecretWhenEnabled(t *testing.T) {
	t.Setenv("UMBRA_AUTH_ENABLED", "true")
	t.Setenv("UMBRA_AUTH_JWT_HS256_SECRET", "")
	_, err := newAuthConfigFromEnv()
	if err == nil {
		t.Fatal("expected auth config error when secret missing")
	}
}

func TestHandlePoliciesAuthEnabledRejectsMissingToken(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth:   &authConfig{Enabled: true, HS256Secret: []byte("test-secret")},
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

func TestHandleReceiptsListAuthEnabledRejectsInsufficientRole(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	secret := []byte("test-secret")
	server := &Server{
		Logger: logger,
		Store:  nil,
		Auth:   &authConfig{Enabled: true, HS256Secret: secret},
	}

	token := mustSignHS256Token(t, secret, map[string]interface{}{
		"sub":       "user-1",
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
