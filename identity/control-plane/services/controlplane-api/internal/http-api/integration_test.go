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
	"time"

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

func TestReceiptExportFiltersAndSafety(t *testing.T) {
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

	tenantID := createTenant(t, db.Pool, "export-tenant")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	now := time.Now().UTC()
	tsAllow := now.Add(-30 * time.Minute)
	tsDeny := now.Add(-2 * time.Hour)

	decisionAllowID := uuid.New()
	decisionDenyID := uuid.New()

	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, ts, decision_id, policy_hash, decision, body_json, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, tsAllow, decisionAllowID, "policy-hash-allow", "allow",
		json.RawMessage(`{"actor":{"id":"user-1"},"tool":{"name":"tool.alpha"},"policy_version":1,"secret":"dont-export"}`),
		nil, "hash-allow", "trace-allow", "span-allow", "req-allow",
	)
	if err != nil {
		t.Fatalf("insert allow decision receipt failed: %v", err)
	}
	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, ts, decision_id, policy_hash, decision, body_json, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, tsDeny, decisionDenyID, "policy-hash-deny", "deny",
		json.RawMessage(`{"actor":{"id":"user-2"},"tool":{"name":"tool.beta"},"policy_version":1,"secret":"dont-export"}`),
		"prev-hash", "hash-deny", "trace-deny", "span-deny", "req-deny",
	)
	if err != nil {
		t.Fatalf("insert deny decision receipt failed: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_invocation(tenant_id, ts, decision_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenantID, tsAllow, decisionAllowID, "tool.alpha", "GET", "/demo", "error", 502, 42,
		json.RawMessage(`{"policy_hash":"policy-hash-allow","policy_version":1,"secret":"dont-export"}`),
		nil, "hash-inv", "trace-inv", "span-inv", "req-allow",
	)
	if err != nil {
		t.Fatalf("insert invocation receipt failed: %v", err)
	}

	from := tsAllow.Add(-10 * time.Minute).Format(time.RFC3339)
	to := tsAllow.Add(10 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/v1/receipts/export?format=json&decision=allow&from="+from+"&to="+to, nil)
	req.Header.Set("x-umbra-tenant-id", tenantID.String())
	w := httptest.NewRecorder()
	server.handleReceiptsExport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("export failed: %d %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if strings.Contains(body, "dont-export") {
		t.Fatalf("export should not include secret payloads")
	}

	var out struct {
		SchemaVersion string                   `json:"schema_version"`
		Items         []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse export response failed: %v", err)
	}
	if out.SchemaVersion == "" {
		t.Fatalf("expected schema_version")
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected 1 receipt in export, got %d", len(out.Items))
	}
	if out.Items[0]["decision"] != "allow" {
		t.Fatalf("expected decision allow, got %v", out.Items[0]["decision"])
	}
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	sqlFiles := []string{"0001_init.sql", "0002_add_request_id.sql", "0003_add_receipt_indexes.sql", "0004_add_receipt_search_indexes.sql", "0005_add_receipt_search_text.sql"}
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
