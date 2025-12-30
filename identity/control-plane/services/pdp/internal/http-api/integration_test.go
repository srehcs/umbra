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

	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
)

func TestPDPUsesActivePolicy(t *testing.T) {
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

	if err := applyMigrationsPDP(t, db.Pool); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `TRUNCATE receipts_decision, receipts_invocation, policies, tools, tenants RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}

	tenantID := createTenantPDP(t, db.Pool, "pdp-tenant")

	denyPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{Effect: "deny", MethodsAny: []string{"GET"}, PathPrefix: "/demo"},
		},
		Default: "deny",
	}
	allowPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{Effect: "allow", MethodsAny: []string{"GET"}, PathPrefix: "/demo"},
		},
		Default: "deny",
	}

	policyID := insertPolicyPDP(t, db.Pool, tenantID, "pdp-policy", denyPolicy, false)
	updatePolicyPDP(t, db.Pool, policyID, allowPolicy)
	activatePolicyPDP(t, db.Pool, policyID)

	os.Setenv("DATABASE_URL", dsn)
	logger := slogDiscard()
	mux := http.NewServeMux()
	registerV0(mux, logger)
	server := httptest.NewServer(mux)
	defer server.Close()

	req := protocol.DecisionRequest{
		Tenant: protocol.TenantContext{TenantID: tenantID.String()},
		Actor:  protocol.Actor{Type: "human", ID: "user-1", Roles: []string{"developer"}},
		Tool:   protocol.Tool{Name: "demo.tool", Method: "GET", Endpoint: "/demo"},
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(server.URL+"/v1/decision", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("pdp request failed: %v", err)
	}
	defer resp.Body.Close()

	var out protocol.DecisionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if out.Decision != "allow" {
		t.Fatalf("expected allow, got %s", out.Decision)
	}
	if out.PolicyHash == "" {
		t.Fatalf("expected policy hash in response")
	}
}

func applyMigrationsPDP(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	sqlFiles := []string{"0001_init.sql", "0002_add_request_id.sql", "0003_add_receipt_indexes.sql", "0004_add_receipt_search_indexes.sql", "0005_add_receipt_search_text.sql"}
	for _, name := range sqlFiles {
		content, err := os.ReadFile(migrationPathPDP(t, name))
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

func migrationPathPDP(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	return filepath.Join(root, "migrations", name)
}

func createTenantPDP(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO tenants(name) VALUES($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("insert tenant failed: %v", err)
	}
	return id
}

func insertPolicyPDP(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID, name string, pol policy.Policy, active bool) uuid.UUID {
	t.Helper()
	policyJSON, _ := json.Marshal(pol)
	hash, _ := policy.ComputePolicyHash(policyJSON)
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `
    INSERT INTO policies(tenant_id, name, version, active, policy_json, policy_hash)
    VALUES ($1,$2,1,$3,$4,$5)
    RETURNING id`, tenant, name, active, policyJSON, hash).Scan(&id); err != nil {
		t.Fatalf("insert policy failed: %v", err)
	}
	return id
}

func updatePolicyPDP(t *testing.T, pool *pgxpool.Pool, policyID uuid.UUID, pol policy.Policy) {
	t.Helper()
	policyJSON, _ := json.Marshal(pol)
	hash, _ := policy.ComputePolicyHash(policyJSON)
	if _, err := pool.Exec(context.Background(), `
    UPDATE policies SET policy_json=$1, policy_hash=$2, version=version+1 WHERE id=$3`, policyJSON, hash, policyID); err != nil {
		t.Fatalf("update policy failed: %v", err)
	}
}

func activatePolicyPDP(t *testing.T, pool *pgxpool.Pool, policyID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE policies SET active=false`); err != nil {
		t.Fatalf("deactivate policies failed: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE policies SET active=true WHERE id=$1`, policyID); err != nil {
		t.Fatalf("activate policy failed: %v", err)
	}
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
