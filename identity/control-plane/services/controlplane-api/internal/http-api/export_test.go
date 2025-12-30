package httpapi

import (
	"encoding/csv"
	"strings"
	"testing"

	"net/http/httptest"

	"github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

func TestReceiptExportCSVHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	writeReceiptsCSV(rec, []storage.ExportRecord{})

	r := csv.NewReader(strings.NewReader(rec.Body.String()))
	header, err := r.Read()
	if err != nil {
		t.Fatalf("read csv header failed: %v", err)
	}
	expected := []string{
		"schema_version",
		"kind",
		"ts",
		"request_id",
		"decision_id",
		"trace_id",
		"policy_hash",
		"policy_version",
		"decision",
		"actor_id",
		"tool_name",
		"method",
		"path",
		"outcome",
		"status_code",
		"receipt_hash",
		"receipt_prev_hash",
	}
	if len(header) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(header))
	}
	for i, name := range expected {
		if header[i] != name {
			t.Fatalf("expected header %q at %d, got %q", name, i, header[i])
		}
	}
}
