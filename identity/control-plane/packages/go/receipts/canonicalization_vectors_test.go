package receipts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type vector struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Input     json.RawMessage `json:"input"`
	Canonical string          `json:"canonical"`
	Hash      string          `json:"hash,omitempty"`
}

type decisionVector struct {
	Actor         actorVector `json:"actor"`
	Tool          toolVector  `json:"tool"`
	Decision      string      `json:"decision"`
	PolicyHash    string      `json:"policy_hash,omitempty"`
	PolicyVersion int         `json:"policy_version,omitempty"`
	RequestID     string      `json:"request_id,omitempty"`
	TraceID       string      `json:"trace_id,omitempty"`
	SpanID        string      `json:"span_id,omitempty"`
}

type actorVector struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Roles []string `json:"roles,omitempty"`
}

type toolVector struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Endpoint string `json:"endpoint"`
}

type invocationVector struct {
	ToolName   string `json:"tool_name"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Outcome    string `json:"outcome"`
	StatusCode *int   `json:"status_code,omitempty"`
	LatencyMs  int    `json:"latency_ms"`
	RequestID  string `json:"request_id,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	SpanID     string `json:"span_id,omitempty"`
}

func TestCanonicalizationVectors(t *testing.T) {
	vectors := loadVectors(t)
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			var canonical []byte
			switch v.Type {
			case "decision_v1":
				var body decisionVector
				mustUnmarshal(t, v.Input, &body)
				canonical = mustCanonical(t, body)
			case "invocation_v1":
				var body invocationVector
				mustUnmarshal(t, v.Input, &body)
				canonical = mustCanonical(t, body)
			default:
				t.Fatalf("unknown vector type %q", v.Type)
			}
			if string(canonical) != v.Canonical {
				t.Fatalf("canonical mismatch\nexpected: %s\ngot:      %s", v.Canonical, string(canonical))
			}
			if v.Hash != "" {
				hash := HashBytes(canonical)
				if hash != v.Hash {
					t.Fatalf("hash mismatch\nexpected: %s\ngot:      %s", v.Hash, hash)
				}
			}
		})
	}
}

func loadVectors(t *testing.T) []vector {
	t.Helper()
	path := vectorPath(t, "vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var out []vector
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("no vectors found")
	}
	return out
}

func vectorPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	return filepath.Join(root, "docs", "test_vectors", "canonicalization", name)
}

func mustUnmarshal(t *testing.T, raw []byte, dst interface{}) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
}

func mustCanonical(t *testing.T, body interface{}) []byte {
	t.Helper()
	out, err := CanonicalJSON(body)
	if err != nil {
		t.Fatalf("canonical json failed: %v", err)
	}
	return out
}
