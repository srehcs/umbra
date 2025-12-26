package receipts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// IMPORTANT: Determinism matters.
// - encoding/json preserves struct field order.
// - map iteration order is randomized in Go.
// Therefore, receipt bodies should use structs (or pre-normalized slices),
// not arbitrary maps, when computing hashes.

type ReceiptEnvelope struct {
	Version   int             `json:"version"`
	Type      string          `json:"type"` // e.g. "decision" | "invocation"
	TenantID  string          `json:"tenant_id"`
	Timestamp string          `json:"timestamp"` // RFC3339
	Body      json.RawMessage `json:"body"`
}

// HashChainFields enable tamper-evident linking.
type HashChainFields struct {
	PrevHash string `json:"prev_hash"`
	Hash     string `json:"hash"`
}

func CanonicalJSON(v interface{}) ([]byte, error) {
	// Only deterministic if v is a struct / stable slices.
	return json.Marshal(v)
}

func HashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
