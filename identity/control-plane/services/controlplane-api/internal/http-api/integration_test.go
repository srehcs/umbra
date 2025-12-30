package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	"github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

func TestPolicyLifecycle_ActivateUpdatesActive(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	db, err := stor.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("db connect failed: %v", err)
	}
	defer db.Close()

	if err := applyMigrations(t, db.Pool); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `TRUNCATE receipts_decision, receipts_invocation, policies, tools, tenants RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}

	tenantID := createTenant(t, db.Pool, "test-tenant-a")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	policyOne := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{Effect: "deny", MethodsAny: []string{"GET"}, PathPrefix: "/demo"},
		},
		Default: "deny",
	}
	policyTwo := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{Effect: "allow", MethodsAny: []string{"GET"}, PathPrefix: "/demo"},
		},
		Default: "deny",
	}

	p1ID := createPolicyViaAPI(t, server, tenantID, "policy-1", policyOne)
	updatePolicyViaAPI(t, server, tenantID, p1ID, policyTwo)
	activatePolicyViaAPI(t, server, tenantID, p1ID)

	active := getActivePolicy(t, server, tenantID)
	if active["id"] != p1ID.String() {
		t.Fatalf("expected active policy %s, got %v", p1ID, active["id"])
	}

	p2ID := createPolicyViaAPI(t, server, tenantID, "policy-2", policyTwo)
	activatePolicyViaAPI(t, server, tenantID, p2ID)

	list := listPoliciesViaAPI(t, server, tenantID)
	activeCount := 0
	for _, p := range list {
		if p.Active {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active policy, got %d", activeCount)
	}
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	sqlFiles := []string{"0001_init.sql", "0002_add_request_id.sql"}
	for _, name := range sqlFiles {
		content, err := os.ReadFile(migrationPath(t, name))
		if err != nil {
			return err
		}
		stmts := strings.Split(string(content), ";")
		for _, stmt := range stmts {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := pool.Exec(context.Background(), stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrationPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	return filepath.Join(root, "migrations", name)
}

func createTenant(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO tenants(name) VALUES($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("insert tenant failed: %v", err)
	}
	return id
}

func createPolicyViaAPI(t *testing.T, server *Server, tenant uuid.UUID, name string, pol policy.Policy) uuid.UUID {
	t.Helper()
	policyJSON, _ := json.Marshal(pol)
	body := map[string]interface{}{
		"name":   name,
		"policy": json.RawMessage(policyJSON),
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/policies", bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handlePolicies(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create policy failed: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse create response: %v", err)
	}
	id, err := uuid.Parse(resp.ID)
	if err != nil {
		t.Fatalf("invalid policy id: %v", err)
	}
	return id
}

func updatePolicyViaAPI(t *testing.T, server *Server, tenant uuid.UUID, policyID uuid.UUID, pol policy.Policy) {
	t.Helper()
	policyJSON, _ := json.Marshal(pol)
	body := map[string]interface{}{
		"policy": json.RawMessage(policyJSON),
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/v1/policies/"+policyID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handlePolicyByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update policy failed: %d %s", w.Code, w.Body.String())
	}
}

func activatePolicyViaAPI(t *testing.T, server *Server, tenant uuid.UUID, policyID uuid.UUID) {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/policies/"+policyID.String()+"/activate", nil)
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handlePolicyByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("activate policy failed: %d %s", w.Code, w.Body.String())
	}
}

func getActivePolicy(t *testing.T, server *Server, tenant uuid.UUID) map[string]string {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/policies/active", nil)
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handleActivePolicy(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get active policy failed: %d %s", w.Code, w.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse active policy response: %v", err)
	}
	return out
}

func listPoliciesViaAPI(t *testing.T, server *Server, tenant uuid.UUID) []policyResponse {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/policies", nil)
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handlePolicies(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list policies failed: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Items []policyResponse `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse list policies response: %v", err)
	}
	return resp.Items
}
