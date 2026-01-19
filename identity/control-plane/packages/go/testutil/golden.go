package testutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	GoldenAny      = "{{any}}"
	GoldenUUID     = "{{uuid}}"
	GoldenNonEmpty = "{{nonempty}}"
	GoldenRFC3339  = "{{rfc3339}}"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var goldenCache = newGoldenCache()

func GoldenPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	return filepath.Join(root, "docs", "test_vectors", "contracts", name)
}

func LoadGoldenBytes(t *testing.T, name string) []byte {
	t.Helper()
	return goldenCache.loadBytes(t, name)
}

func LoadGolden(t *testing.T, name string) interface{} {
	t.Helper()
	return goldenCache.loadParsed(t, name)
}

func LoadGoldenInto(t *testing.T, name string, dst interface{}) {
	t.Helper()
	data := LoadGoldenBytes(t, name)
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
}

func AssertGoldenMatch(t *testing.T, expected, actual interface{}) {
	t.Helper()
	assertGoldenMatch(t, expected, actual, "$", MatchOptions{})
}

type MatchOptions struct {
	Strict bool
}

func AssertGoldenMatchWithOptions(t *testing.T, expected, actual interface{}, opts MatchOptions) {
	t.Helper()
	if opts.Strict {
		if expectedText, ok := rawNoPlaceholder(expected); ok {
			if actualText, ok := rawNoPlaceholder(actual); ok {
				if expectedText != actualText {
					t.Fatalf("strict match failed: expected %s got %s", expectedText, actualText)
				}
				return
			}
		}
	}
	assertGoldenMatch(t, expected, actual, "$", opts)
}

func assertGoldenMatch(t *testing.T, expected, actual interface{}, path string, opts MatchOptions) {
	t.Helper()
	switch exp := expected.(type) {
	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			t.Fatalf("expected object at %s, got %T", path, actual)
		}
		if opts.Strict && len(act) != len(exp) {
			t.Fatalf("object key mismatch at %s: expected %d got %d", path, len(exp), len(act))
		}
		for key, expVal := range exp {
			actVal, ok := act[key]
			if !ok {
				t.Fatalf("missing key %s.%s", path, key)
			}
			assertGoldenMatch(t, expVal, actVal, path+"."+key, opts)
		}
		if opts.Strict {
			for key := range act {
				if _, ok := exp[key]; !ok {
					t.Fatalf("unexpected key %s.%s", path, key)
				}
			}
		}
	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok {
			t.Fatalf("expected array at %s, got %T", path, actual)
		}
		if len(act) != len(exp) {
			t.Fatalf("array length mismatch at %s: expected %d got %d", path, len(exp), len(act))
		}
		for i, expVal := range exp {
			assertGoldenMatch(t, expVal, act[i], path+indexSuffix(i), opts)
		}
	case string:
		if isPlaceholder(exp) {
			assertPlaceholder(t, exp, actual, path)
			return
		}
		act, ok := actual.(string)
		if !ok {
			t.Fatalf("expected string at %s, got %T", path, actual)
		}
		if act != exp {
			t.Fatalf("string mismatch at %s: expected %q got %q", path, exp, act)
		}
	case float64:
		act, ok := actual.(float64)
		if !ok {
			t.Fatalf("expected number at %s, got %T", path, actual)
		}
		if act != exp {
			t.Fatalf("number mismatch at %s: expected %v got %v", path, exp, act)
		}
	case bool:
		act, ok := actual.(bool)
		if !ok {
			t.Fatalf("expected bool at %s, got %T", path, actual)
		}
		if act != exp {
			t.Fatalf("bool mismatch at %s: expected %v got %v", path, exp, act)
		}
	case nil:
		if actual != nil {
			t.Fatalf("expected null at %s, got %T", path, actual)
		}
	default:
		t.Fatalf("unsupported expected type %T at %s", expected, path)
	}
}

func AssertJSONBytesMatch(t *testing.T, golden, actual []byte, strict bool) {
	t.Helper()
	if strict && !bytes.Contains(golden, []byte("{{")) {
		compExpected := compactJSON(t, golden)
		compActual := compactJSON(t, actual)
		if !bytes.Equal(compExpected, compActual) {
			t.Fatalf("strict json mismatch: expected %s got %s", compExpected, compActual)
		}
		return
	}
	var expectedVal interface{}
	if err := json.Unmarshal(golden, &expectedVal); err != nil {
		t.Fatalf("parse golden json: %v", err)
	}
	var actualVal interface{}
	if err := json.Unmarshal(actual, &actualVal); err != nil {
		t.Fatalf("parse actual json: %v", err)
	}
	AssertGoldenMatchWithOptions(t, expectedVal, actualVal, MatchOptions{Strict: strict})
}

func AssertStringMatch(t *testing.T, expected, actual, path string) {
	t.Helper()
	if isPlaceholder(expected) {
		assertPlaceholder(t, expected, actual, path)
		return
	}
	if actual != expected {
		t.Fatalf("string mismatch at %s: expected %q got %q", path, expected, actual)
	}
}

func isPlaceholder(value string) bool {
	return value == GoldenAny || value == GoldenUUID || value == GoldenNonEmpty || value == GoldenRFC3339
}

func assertPlaceholder(t *testing.T, placeholder string, actual interface{}, path string) {
	t.Helper()
	if placeholder == GoldenAny {
		if actual == nil {
			t.Fatalf("expected any value at %s, got nil", path)
		}
		return
	}
	act, ok := actual.(string)
	if !ok {
		t.Fatalf("expected string at %s for placeholder %s, got %T", path, placeholder, actual)
	}
	switch placeholder {
	case GoldenUUID:
		if !uuidPattern.MatchString(act) {
			t.Fatalf("expected uuid at %s, got %q", path, act)
		}
	case GoldenNonEmpty:
		if act == "" {
			t.Fatalf("expected non-empty string at %s", path)
		}
	case GoldenRFC3339:
		if _, err := time.Parse(time.RFC3339, act); err != nil {
			t.Fatalf("expected rfc3339 time at %s, got %q", path, act)
		}
	default:
		t.Fatalf("unknown placeholder %s at %s", placeholder, path)
	}
}

func indexSuffix(i int) string {
	return "[" + strconv.Itoa(i) + "]"
}

func rawNoPlaceholder(value interface{}) (string, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(v))
		for key, val := range v {
			if hasPlaceholder(val) {
				return "", false
			}
			clone[key] = val
		}
		b, err := json.Marshal(clone)
		if err != nil {
			return "", false
		}
		return string(b), true
	case []interface{}:
		for _, item := range v {
			if hasPlaceholder(item) {
				return "", false
			}
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(b), true
	default:
		if hasPlaceholder(value) {
			return "", false
		}
		b, err := json.Marshal(value)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

func hasPlaceholder(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return isPlaceholder(v)
	case map[string]interface{}:
		for _, val := range v {
			if hasPlaceholder(val) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if hasPlaceholder(item) {
				return true
			}
		}
	}
	return false
}

type goldenCacheStore struct {
	mu     sync.RWMutex
	bytes  map[string][]byte
	parsed map[string]interface{}
}

func newGoldenCache() *goldenCacheStore {
	return &goldenCacheStore{
		bytes:  make(map[string][]byte),
		parsed: make(map[string]interface{}),
	}
}

func (g *goldenCacheStore) loadBytes(t *testing.T, name string) []byte {
	t.Helper()
	g.mu.RLock()
	if data, ok := g.bytes[name]; ok {
		g.mu.RUnlock()
		return data
	}
	g.mu.RUnlock()

	data, err := os.ReadFile(GoldenPath(t, name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	g.mu.Lock()
	g.bytes[name] = data
	g.mu.Unlock()
	return data
}

func (g *goldenCacheStore) loadParsed(t *testing.T, name string) interface{} {
	t.Helper()
	g.mu.RLock()
	if cached, ok := g.parsed[name]; ok {
		g.mu.RUnlock()
		return cached
	}
	g.mu.RUnlock()

	data := g.loadBytes(t, name)
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse golden %s: %v", name, err)
	}
	g.mu.Lock()
	g.parsed[name] = out
	g.mu.Unlock()
	return out
}

func compactJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compact json: %v", err)
	}
	return buf.Bytes()
}
