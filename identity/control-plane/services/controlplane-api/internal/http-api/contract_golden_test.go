package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/testutil"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

func TestControlPlaneContractGolden(t *testing.T) {
	noStoreServer := &Server{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))}
	handler := http.HandlerFunc(noStoreServer.handleReceipts)
	testutil.RunContractSuite(t, testutil.ContractSuite{
		Name:    "controlplane",
		Handler: handler,
		Cases: []testutil.ContractCase{
			{
				Name:        "receipts storage unavailable",
				Method:      http.MethodGet,
				Path:        "/v1/receipts",
				RequestID:   "req-ctrl-1",
				WantStatus:  http.StatusServiceUnavailable,
				WantHeaders: map[string]string{"x-umbra-request-id": "req-ctrl-1"},
				WantError: &testutil.ErrorExpectation{
					Code:      "STORAGE_UNAVAILABLE",
					Message:   "storage not configured",
					RequestID: "req-ctrl-1",
				},
				Strict:     true,
				AssertBody: assertGoldenError("controlplane_receipts_error.json"),
			},
			{
				Name:       "receipts storage unavailable auto request id",
				Method:     http.MethodGet,
				Path:       "/v1/receipts",
				WantStatus: http.StatusServiceUnavailable,
				WantHeaders: map[string]string{
					"x-umbra-request-id": testutil.GoldenNonEmpty,
				},
				WantError: &testutil.ErrorExpectation{
					Code:    "STORAGE_UNAVAILABLE",
					Message: "storage not configured",
				},
				Strict: true,
			},
		},
	})

	dsn := strings.TrimSpace(os.Getenv("UMBRA_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("UMBRA_TEST_DATABASE_URL not set")
	}
	db, cleanup := testutil.ConnectIsolatedTestDB(t, dsn)
	defer cleanup()
	if err := testutil.ApplySchemaForTests(t, db.Pool); err != nil {
		t.Fatalf("schema setup failed: %v", err)
	}
	tenantID := createTenantForContract(t, db.Pool, "contract-tenant")
	storeServer := &Server{Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Store: dbstore.New(db)}
	storeHandler := http.HandlerFunc(storeServer.handleReceipts)
	fixtures := testutil.NewContractFixtures(
		tenantID.String(),
		"",
		map[string]string{"content-type": "application/json"},
	)
	testutil.RunContractSuite(t, testutil.ContractSuite{
		Name:    "controlplane receipts ingest",
		Handler: storeHandler,
		Cases: []testutil.ContractCase{
			{
				Name:       "receipt ingest created",
				Method:     http.MethodPost,
				Path:       "/v1/receipts",
				RequestID:  "req-receipt-1",
				Fixtures:   fixtures,
				Body:       receiptIngestDecisionBody("req-receipt-1", "decision-1"),
				WantStatus: http.StatusCreated,
				WantHeaders: map[string]string{
					"x-umbra-request-id": "req-receipt-1",
				},
				Strict:     true,
				AssertBody: assertReceiptIngestGolden("controlplane_receipt_ingest_created.json"),
			},
			{
				Name:       "receipt ingest replay",
				Method:     http.MethodPost,
				Path:       "/v1/receipts",
				RequestID:  "req-receipt-1",
				Fixtures:   fixtures,
				Body:       receiptIngestDecisionBody("req-receipt-1", "decision-1"),
				WantStatus: http.StatusOK,
				WantHeaders: map[string]string{
					"x-umbra-request-id": "req-receipt-1",
				},
				Strict:     true,
				AssertBody: assertReceiptIngestGolden("controlplane_receipt_ingest_replay.json"),
			},
			{
				Name:       "receipt ingest conflict",
				Method:     http.MethodPost,
				Path:       "/v1/receipts",
				RequestID:  "req-receipt-1",
				Fixtures:   fixtures,
				Body:       receiptIngestDecisionBody("req-receipt-1", "decision-2"),
				WantStatus: http.StatusConflict,
				WantHeaders: map[string]string{
					"x-umbra-request-id": "req-receipt-1",
				},
				WantError: &testutil.ErrorExpectation{
					Code:      "CONFLICT",
					Message:   "request_id already used with different payload",
					RequestID: "req-receipt-1",
				},
				Strict:     true,
				AssertBody: assertGoldenError("controlplane_receipt_ingest_conflict.json"),
			},
		},
	})
}

func receiptIngestDecisionBody(requestID, decisionID string) []byte {
	return []byte(`{"kind":"decision","request_id":"` + requestID + `","decision_id":"` + decisionID + `","decision":"allow","policy_hash":"policy-hash","trace_id":"trace-1","span_id":"span-1","body":{"actor":{"id":"user-1","type":"human"},"tool":{"name":"demo.tool","method":"GET","endpoint":"/demo"}}}`)
}

func createTenantForContract(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `INSERT INTO tenants(name) VALUES($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("insert tenant failed: %v", err)
	}
	return id
}

func assertReceiptIngestGolden(name string) func(t *testing.T, body []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		var expected testutil.ReceiptIngestResponse
		testutil.LoadGoldenInto(t, name, &expected)
		var actual testutil.ReceiptIngestResponse
		if err := json.Unmarshal(body, &actual); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		testutil.AssertReceiptIngestResponse(t, expected, actual)
	}
}

func assertGoldenError(name string) func(t *testing.T, body []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		var expected protocol.ErrorResponse
		testutil.LoadGoldenInto(t, name, &expected)
		var actual protocol.ErrorResponse
		if err := json.Unmarshal(body, &actual); err != nil {
			t.Fatalf("decode error response: %v", err)
		}
		testutil.AssertErrorEnvelope(t, actual, expected.Error.Code, expected.Error.Message, expected.RequestID, expected.DecisionID, expected.TraceID)
	}
}
