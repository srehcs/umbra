package receipts

import "testing"

func TestCanonicalJSONBytes_SortsKeys(t *testing.T) {
	raw := []byte(`{"b":1,"a":{"d":4,"c":3},"arr":[{"z":2,"y":1},2]}`)
	out, err := CanonicalJSONBytes(raw)
	if err != nil {
		t.Fatalf("canonicalize raw: %v", err)
	}
	expected := `{"a":{"c":3,"d":4},"arr":[{"y":1,"z":2},2],"b":1}`
	if string(out) != expected {
		t.Fatalf("canonical mismatch\nexpected: %s\ngot:      %s", expected, string(out))
	}
}

func TestCanonicalJSONBytes_RejectsFloats(t *testing.T) {
	_, err := CanonicalJSONBytes([]byte(`{"a":1.25}`))
	if err == nil {
		t.Fatalf("expected float rejection")
	}
}

func TestCanonicalJSONBytes_RejectsNonASCII(t *testing.T) {
	_, err := CanonicalJSONBytes([]byte(`{"a":"caf\u00e9"}`))
	if err == nil {
		t.Fatalf("expected non-ascii rejection")
	}
}
