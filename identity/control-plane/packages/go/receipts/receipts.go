package receipts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
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

var (
	ErrCanonicalJSONNonASCII    = errors.New("canonical json contains non-ascii")
	ErrCanonicalJSONFloat       = errors.New("canonical json contains float")
	ErrCanonicalJSONTrailing    = errors.New("canonical json has trailing data")
	ErrCanonicalJSONUnsupported = errors.New("canonical json has unsupported type")
)

// CanonicalJSONBytes canonicalizes raw JSON by sorting object keys and
// preserving numeric string values from json.Number.
func CanonicalJSONBytes(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := encodeCanonicalJSON(&buf, value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ensureDecoderEOF(decoder *json.Decoder) error {
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return ErrCanonicalJSONTrailing
		}
		return err
	}
	return nil
}

func encodeCanonicalJSON(buf *bytes.Buffer, value interface{}) error {
	switch v := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for key := range v {
			if !isASCII(key) {
				return ErrCanonicalJSONNonASCII
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeJSONString(buf, key); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := encodeCanonicalJSON(buf, v[key]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []interface{}:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeCanonicalJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case string:
		if !isASCII(v) {
			return ErrCanonicalJSONNonASCII
		}
		if err := writeJSONString(buf, v); err != nil {
			return err
		}
	case json.Number:
		if err := validateJSONNumber(v); err != nil {
			return err
		}
		buf.WriteString(v.String())
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		return ErrCanonicalJSONUnsupported
	}
	return nil
}

func writeJSONString(buf *bytes.Buffer, value string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	buf.Write(encoded)
	return nil
}

func validateJSONNumber(value json.Number) error {
	raw := value.String()
	if strings.ContainsAny(raw, ".eE") {
		return ErrCanonicalJSONFloat
	}
	return nil
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
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
