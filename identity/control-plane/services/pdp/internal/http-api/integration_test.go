package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
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
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/testutil"
)

func TestPDPUsesActivePolicy(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
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

func TestDecisionReceiptTraceID(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}
	tenantID := createTenantPDP(t, db.Pool, "pdp-trace-tenant")
	allowPolicy := policy.Policy{
		Version: 1,
		Mode:    "abac_v0",
		Rules: []policy.Rule{
			{Effect: "allow", MethodsAny: []string{"GET"}, PathPrefix: "/demo"},
		},
		Default: "deny",
	}
	policyID := insertPolicyPDP(t, db.Pool, tenantID, "trace-policy", allowPolicy, false)
	activatePolicyPDP(t, db.Pool, policyID)

	os.Setenv("DATABASE_URL", dsn)
	logger := slogDiscard()
	mux := http.NewServeMux()
	registerV0(mux, logger)
	server := httptest.NewServer(mux)
	defer server.Close()

	traceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	spanID := "00f067aa0ba902b7"
	requestID := "req-trace-001"

	req := protocol.DecisionRequest{
		Tenant: protocol.TenantContext{TenantID: tenantID.String()},
		Actor:  protocol.Actor{Type: "human", ID: "user-1", Roles: []string{"developer"}},
		Tool:   protocol.Tool{Name: "demo.tool", Method: "GET", Endpoint: "/demo"},
		Trace: &protocol.TraceContext{
			RequestID: requestID,
			TraceID:   traceID,
			SpanID:    spanID,
		},
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(server.URL+"/v1/decision", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("pdp request failed: %v", err)
	}
	defer resp.Body.Close()

	var storedTraceID, storedSpanID string
	if err := db.Pool.QueryRow(ctx, `
    SELECT trace_id, span_id
    FROM receipts_decision
    WHERE request_id=$1
    ORDER BY ts DESC
    LIMIT 1`, requestID).Scan(&storedTraceID, &storedSpanID); err != nil {
		t.Fatalf("trace lookup failed: %v", err)
	}
	if storedTraceID != traceID {
		t.Fatalf("expected trace_id %s, got %s", traceID, storedTraceID)
	}
	if storedSpanID != spanID {
		t.Fatalf("expected span_id %s, got %s", spanID, storedSpanID)
	}
}

func applySchema(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	return testutil.ApplySchemaForTests(t, pool)
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
