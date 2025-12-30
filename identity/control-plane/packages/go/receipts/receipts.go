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

const (
	VerifyHashMismatch = "HASH_MISMATCH"
	VerifyMissingLink  = "MISSING_LINK"
	VerifyOutOfOrder   = "OUT_OF_ORDER"
)

type ChainRecord struct {
	ID       string          `json:"id"`
	Body     json.RawMessage `json:"body"`
	PrevHash string          `json:"prev_hash"`
	Hash     string          `json:"hash"`
}

type VerifyFailure struct {
	ReceiptID string `json:"receipt_id"`
	Code      string `json:"code"`
}

type VerifyResult struct {
	OK      bool           `json:"ok"`
	Checked int            `json:"checked"`
	Failure *VerifyFailure `json:"failure,omitempty"`
}

func CanonicalJSON(v interface{}) ([]byte, error) {
	// Only deterministic if v is a struct / stable slices.
	return json.Marshal(v)
}

func HashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// VerifyChain checks receipts ordered from oldest to newest.
// The first record is treated as the chain anchor for the window.
func VerifyChain(records []ChainRecord) VerifyResult {
	if len(records) == 0 {
		return VerifyResult{OK: true, Checked: 0}
	}
	hashIndex := make(map[string]int, len(records))
	for i, r := range records {
		if r.Hash != "" {
			hashIndex[r.Hash] = i
		}
	}

	for i, r := range records {
		expected := HashBytes(append([]byte(r.PrevHash), r.Body...))
		if expected != r.Hash {
			return VerifyResult{
				OK:      false,
				Checked: i + 1,
				Failure: &VerifyFailure{ReceiptID: r.ID, Code: VerifyHashMismatch},
			}
		}
		if i == 0 {
			continue
		}
		if r.PrevHash == records[i-1].Hash {
			continue
		}
		if r.PrevHash == "" {
			return VerifyResult{
				OK:      false,
				Checked: i + 1,
				Failure: &VerifyFailure{ReceiptID: r.ID, Code: VerifyMissingLink},
			}
		}
		if idx, ok := hashIndex[r.PrevHash]; ok {
			code := VerifyMissingLink
			if idx > i-1 {
				code = VerifyOutOfOrder
			}
			return VerifyResult{
				OK:      false,
				Checked: i + 1,
				Failure: &VerifyFailure{ReceiptID: r.ID, Code: code},
			}
		}
		return VerifyResult{
			OK:      false,
			Checked: i + 1,
			Failure: &VerifyFailure{ReceiptID: r.ID, Code: VerifyMissingLink},
		}
	}

	return VerifyResult{OK: true, Checked: len(records)}
}
