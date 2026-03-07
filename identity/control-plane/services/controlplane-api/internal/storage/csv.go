package storage

import (
	"encoding/csv"
	"strconv"
	"time"
)

func WriteReceiptsCSVHeader(writer *csv.Writer) {
	_ = writer.Write([]string{
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
		"signature_alg",
		"signature_kid",
		"signature",
		"signed_at",
		"receipt_hash",
		"receipt_prev_hash",
	})
}

func WriteReceiptsCSVRecord(writer *csv.Writer, item ExportRecord) {
	_ = writer.Write([]string{
		item.SchemaVersion,
		item.Kind,
		item.TS.UTC().Format(time.RFC3339),
		item.RequestID,
		item.DecisionID,
		item.TraceID,
		item.PolicyHash,
		intPtrToString(item.PolicyVersion),
		item.Decision,
		item.ActorID,
		item.ToolName,
		item.Method,
		item.Path,
		item.Outcome,
		intPtrToString(item.StatusCode),
		item.SignatureAlg,
		item.SignatureKid,
		item.Signature,
		timePtrToString(item.SignedAt),
		item.ReceiptHash,
		item.ReceiptPrevHash,
	})
}

func intPtrToString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func timePtrToString(v *time.Time) string {
	if v == nil || v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
