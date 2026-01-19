package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/testutil"
	"github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

func TestPolicyLifecycle_ActivateUpdatesActive(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
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
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}

	tenantID := createTenant(t, db.Pool, "export-tenant")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	now := time.Now().UTC()
	tsAllow := now.Add(-30 * time.Minute)
	tsDeny := now.Add(-2 * time.Hour)

	decisionAllowID := uuid.New()
	decisionDenyID := uuid.New()

	var err error
	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, ts, decision_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		tenantID, tsAllow, decisionAllowID, "policy-hash-allow", "allow",
		json.RawMessage(`{"actor":{"id":"user-1"},"tool":{"name":"tool.alpha"},"policy_version":1,"secret":"dont-export"}`),
		json.RawMessage(`{"actor":{"id":"user-1"},"tool":{"name":"tool.alpha"},"policy_version":1,"secret":"dont-export"}`),
		nil, "hash-allow", "trace-allow", "span-allow", "req-allow",
	)
	if err != nil {
		t.Fatalf("insert allow decision receipt failed: %v", err)
	}
	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, ts, decision_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		tenantID, tsDeny, decisionDenyID, "policy-hash-deny", "deny",
		json.RawMessage(`{"actor":{"id":"user-2"},"tool":{"name":"tool.beta"},"policy_version":1,"secret":"dont-export"}`),
		json.RawMessage(`{"actor":{"id":"user-2"},"tool":{"name":"tool.beta"},"policy_version":1,"secret":"dont-export"}`),
		"prev-hash", "hash-deny", "trace-deny", "span-deny", "req-deny",
	)
	if err != nil {
		t.Fatalf("insert deny decision receipt failed: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
    INSERT INTO receipts_invocation(tenant_id, ts, decision_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, body_canonical, prev_hash, hash, trace_id, span_id, request_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, tsAllow, decisionAllowID, "tool.alpha", "GET", "/demo", "error", 502, 42,
		json.RawMessage(`{"policy_hash":"policy-hash-allow","policy_version":1,"secret":"dont-export"}`),
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

func TestReceiptIngestDecisionChain(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}

	tenantID := createTenant(t, db.Pool, "receipt-ingest-tenant")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	body := map[string]interface{}{
		"actor": map[string]interface{}{"id": "user-1", "type": "human"},
		"tool":  map[string]interface{}{"name": "demo.tool", "method": "GET", "endpoint": "/demo"},
	}
	bodyBytes, _ := json.Marshal(body)

	first := receiptIngestRequest{
		Kind:       "decision",
		RequestID:  "req-1",
		DecisionID: uuid.NewString(),
		Decision:   "allow",
		PolicyHash: "policy-hash-1",
		Body:       bodyBytes,
	}
	resp1 := ingestReceipt(t, server, tenantID, first)
	if resp1.Hash == "" {
		t.Fatalf("expected hash in response")
	}

	second := receiptIngestRequest{
		Kind:       "decision",
		RequestID:  "req-2",
		DecisionID: uuid.NewString(),
		Decision:   "deny",
		PolicyHash: "policy-hash-2",
		Body:       bodyBytes,
	}
	resp2 := ingestReceipt(t, server, tenantID, second)
	if resp2.PrevHash != resp1.Hash {
		t.Fatalf("expected prev_hash %s, got %s", resp1.Hash, resp2.PrevHash)
	}
}

func TestReceiptIngestIdempotency(t *testing.T) {
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

	tenantID := createTenant(t, db.Pool, "receipt-idem-tenant")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	body := map[string]interface{}{
		"actor": map[string]interface{}{"id": "user-1", "type": "human"},
		"tool":  map[string]interface{}{"name": "demo.tool", "method": "GET", "endpoint": "/demo"},
	}
	bodyBytes, _ := json.Marshal(body)

	base := receiptIngestRequest{
		Kind:       "decision",
		RequestID:  "req-idem-1",
		DecisionID: uuid.NewString(),
		Decision:   "allow",
		PolicyHash: "policy-hash-1",
		Body:       bodyBytes,
	}

	first := ingestReceiptRaw(t, server, tenantID, base)
	if first.Code != http.StatusCreated {
		t.Fatalf("receipt ingest failed: %d %s", first.Code, first.Body.String())
	}
	var firstResp receiptIngestResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("parse receipt ingest response: %v", err)
	}

	second := ingestReceiptRaw(t, server, tenantID, base)
	if second.Code != http.StatusOK {
		t.Fatalf("expected idempotent replay, got %d %s", second.Code, second.Body.String())
	}
	var secondResp receiptIngestResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("parse replay response: %v", err)
	}
	if secondResp.ReceiptID != firstResp.ReceiptID || secondResp.Hash != firstResp.Hash {
		t.Fatalf("expected replay to return original receipt")
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `
    SELECT COUNT(*)
    FROM receipts_decision
    WHERE tenant_id=$1 AND request_id=$2`, tenantID, base.RequestID).Scan(&count); err != nil {
		t.Fatalf("count receipts failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 receipt, got %d", count)
	}

	conflict := base
	conflict.Decision = "deny"
	third := ingestReceiptRaw(t, server, tenantID, conflict)
	if third.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d %s", third.Code, third.Body.String())
	}
	var errResp protocol.ErrorResponse
	if err := json.Unmarshal(third.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("parse conflict response: %v", err)
	}
	if errResp.Error.Code != protocol.ErrorCodeConflict {
		t.Fatalf("expected conflict error code, got %s", errResp.Error.Code)
	}
}

func TestReceiptIngestIdempotencyCanonicalBody(t *testing.T) {
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

	tenantID := createTenant(t, db.Pool, "receipt-idem-canonical")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	bodyA := json.RawMessage(`{"b":2,"a":{"d":4,"c":3}}`)
	bodyB := json.RawMessage(`{"a":{"c":3,"d":4},"b":2}`)

	base := receiptIngestRequest{
		Kind:      "invocation",
		RequestID: "req-idem-canonical-1",
		ToolName:  "demo.tool",
		Method:    "GET",
		Path:      "/demo",
		Outcome:   "success",
		LatencyMs: intPtr(12),
		Body:      bodyA,
	}

	first := ingestReceiptRaw(t, server, tenantID, base)
	if first.Code != http.StatusCreated {
		t.Fatalf("receipt ingest failed: %d %s", first.Code, first.Body.String())
	}
	var firstResp receiptIngestResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("parse receipt ingest response: %v", err)
	}

	replay := base
	replay.Body = bodyB
	second := ingestReceiptRaw(t, server, tenantID, replay)
	if second.Code != http.StatusOK {
		t.Fatalf("expected idempotent replay, got %d %s", second.Code, second.Body.String())
	}
	var secondResp receiptIngestResponse
	if err := json.Unmarshal(second.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("parse replay response: %v", err)
	}
	if secondResp.ReceiptID != firstResp.ReceiptID || secondResp.Hash != firstResp.Hash {
		t.Fatalf("expected replay to return original receipt")
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `
    SELECT COUNT(*)
    FROM receipts_invocation
    WHERE tenant_id=$1 AND request_id=$2`, tenantID, base.RequestID).Scan(&count); err != nil {
		t.Fatalf("count receipts failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 receipt, got %d", count)
	}
}

func intPtr(v int) *int {
	return &v
}

func TestReceiptIngestIdempotencyConcurrent(t *testing.T) {
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

	tenantID := createTenant(t, db.Pool, "receipt-idem-concurrent")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	reqBody := json.RawMessage(`{"actor":{"id":"user-1","type":"human"}}`)
	base := receiptIngestRequest{
		Kind:       "decision",
		RequestID:  "req-idem-concurrent-1",
		DecisionID: uuid.NewString(),
		Decision:   "allow",
		PolicyHash: "policy-hash-1",
		Body:       reqBody,
	}
	bodyBytes, _ := json.Marshal(base)

	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			<-start
			results <- ingestReceiptRawBytes(server, tenantID, bodyBytes)
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	statuses := map[int]int{}
	for resp := range results {
		statuses[resp.Code]++
	}
	if statuses[http.StatusCreated] != 1 || statuses[http.StatusOK] != 1 {
		t.Fatalf("expected one created and one replay, got %+v", statuses)
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `
    SELECT COUNT(*)
    FROM receipts_decision
    WHERE tenant_id=$1 AND request_id=$2`, tenantID, base.RequestID).Scan(&count); err != nil {
		t.Fatalf("count receipts failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 receipt, got %d", count)
	}
}

func TestReceiptIngestRejectsInvalidJSONNumbers(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()

	if err := applySchema(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}

	tenantID := createTenant(t, db.Pool, "receipt-invalid-json")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &Server{Logger: logger, Store: storage.New(db)}

	base := receiptIngestRequest{
		Kind:       "decision",
		RequestID:  "req-invalid-json-1",
		DecisionID: uuid.NewString(),
		Decision:   "allow",
		PolicyHash: "policy-hash-1",
	}

	cases := []struct {
		name string
		body json.RawMessage
	}{
		{name: "float", body: json.RawMessage(`{"latency_ms":1.25}`)},
		{name: "non_ascii", body: json.RawMessage(`{"msg":"caf\u00e9"}`)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := base
			req.RequestID = base.RequestID + "-" + tc.name
			req.Body = tc.body
			resp := ingestReceiptRaw(t, server, tenantID, req)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("expected bad request, got %d %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func applySchema(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	return testutil.ApplySchemaForTests(t, pool)
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

func ingestReceipt(t *testing.T, server *Server, tenant uuid.UUID, body receiptIngestRequest) receiptIngestResponse {
	t.Helper()
	w := ingestReceiptRaw(t, server, tenant, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("receipt ingest failed: %d %s", w.Code, w.Body.String())
	}
	var out receiptIngestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse receipt ingest response: %v", err)
	}
	return out
}

func ingestReceiptRaw(t *testing.T, server *Server, tenant uuid.UUID, body receiptIngestRequest) *httptest.ResponseRecorder {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	return ingestReceiptRawBytes(server, tenant, bodyBytes)
}

func ingestReceiptRawBytes(server *Server, tenant uuid.UUID, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/v1/receipts", bytes.NewReader(body))
	req.Header.Set("x-umbra-tenant-id", tenant.String())
	w := httptest.NewRecorder()
	server.handleReceipts(w, req)
	return w
}
