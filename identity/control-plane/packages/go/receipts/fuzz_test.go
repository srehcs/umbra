package receipts

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzCanonicalJSONBytesDeterministic(f *testing.F) {
	f.Add([]byte(`{"b":2,"a":1}`))
	f.Add([]byte(`{"list":[{"z":3,"a":1},true,null]}`))
	f.Add([]byte(`["a","b","c"]`))
	f.Add([]byte(`{"nested":{"inner":{"k":"v"}}}`))
	f.Add([]byte(`{"a":1.25}`))
	f.Add([]byte(`{"a":"caf\u00e9"}`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		if !json.Valid(raw) {
			return
		}
		value, err := decodeJSONValue(raw)
		if err != nil {
			t.Fatalf("unexpected canonicalization error: %v", err)
		}
		hasNonASCII, hasFloat := detectDisallowed(value)
		out, err := CanonicalJSONBytes(raw)
		if hasNonASCII || hasFloat {
			if err == nil {
				t.Fatalf("expected disallowed json error (non-ascii=%t float=%t)", hasNonASCII, hasFloat)
			}
			return
		}
		if err != nil {
			return
		}
		if !json.Valid(out) {
			t.Fatalf("canonical output is not valid JSON: %s", out)
		}
		if !isASCIIBytes(out) {
			t.Fatalf("canonical output contains non-ascii bytes: %s", out)
		}
		again, err := CanonicalJSONBytes(out)
		if err != nil {
			t.Fatalf("canonical output re-parse failed: %v", err)
		}
		if !bytes.Equal(out, again) {
			t.Fatalf("canonical output not idempotent: %s vs %s", out, again)
		}
	})
}

func FuzzCanonicalizeIdempotencyPayloadDeterministic(f *testing.F) {
	f.Add(
		"invocation",
		"req-123",
		"dec-123",
		"allow",
		"policy-hash",
		"tool-name",
		"POST",
		"/v1/tools",
		"ok",
		[]byte(`{"a":1}`),
	)

	f.Fuzz(func(t *testing.T, kind, requestID, decisionID, decision, policyHash, toolName, method, path, outcome string, body []byte) {
		if !json.Valid(body) {
			body = []byte("null")
		}
		payload := IdempotencyPayload{
			Kind:       kind,
			RequestID:  requestID,
			DecisionID: decisionID,
			Decision:   decision,
			PolicyHash: policyHash,
			ToolName:   toolName,
			Method:     method,
			Path:       path,
			Outcome:    outcome,
			Body:       body,
		}

		first, err := CanonicalizeIdempotencyPayload(payload)
		if err != nil {
			t.Fatalf("canonicalize payload failed: %v", err)
		}
		second, err := CanonicalizeIdempotencyPayload(payload)
		if err != nil {
			t.Fatalf("canonicalize payload repeat failed: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("canonicalize payload not deterministic: %s vs %s", first, second)
		}
		if !json.Valid(first) {
			t.Fatalf("canonicalized payload invalid JSON: %s", first)
		}
	})
}

func isASCIIBytes(value []byte) bool {
	for _, b := range value {
		if b >= 0x80 {
			return false
		}
	}
	return true
}

func decodeJSONValue(raw []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func detectDisallowed(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		hasNonASCII := false
		hasFloat := false
		for key, item := range v {
			if !isASCIIString(key) {
				hasNonASCII = true
			}
			itemNonASCII, itemFloat := detectDisallowed(item)
			hasNonASCII = hasNonASCII || itemNonASCII
			hasFloat = hasFloat || itemFloat
		}
		return hasNonASCII, hasFloat
	case []interface{}:
		hasNonASCII := false
		hasFloat := false
		for _, item := range v {
			itemNonASCII, itemFloat := detectDisallowed(item)
			hasNonASCII = hasNonASCII || itemNonASCII
			hasFloat = hasFloat || itemFloat
		}
		return hasNonASCII, hasFloat
	case string:
		return !isASCIIString(v), false
	case json.Number:
		raw := v.String()
		for i := 0; i < len(raw); i++ {
			switch raw[i] {
			case '.', 'e', 'E':
				return false, true
			}
		}
		return false, false
	default:
		return false, false
	}
}

func isASCIIString(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
}
