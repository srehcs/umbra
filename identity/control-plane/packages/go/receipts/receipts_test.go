package receipts

import (
	"testing"
)

type testBody struct {
	Msg string `json:"msg"`
}

func buildRecord(t *testing.T, id string, prev string, msg string) ChainRecord {
	t.Helper()
	bodyBytes, err := CanonicalJSON(testBody{Msg: msg})
	if err != nil {
		t.Fatalf("canonical json failed: %v", err)
	}
	hash := HashBytes(append([]byte(prev), bodyBytes...))
	return ChainRecord{
		ID:       id,
		Body:     bodyBytes,
		PrevHash: prev,
		Hash:     hash,
	}
}

func TestVerifyChain_HashMismatch(t *testing.T) {
	r1 := buildRecord(t, "r1", "", "one")
	r2 := buildRecord(t, "r2", r1.Hash, "two")
	r2.Body = []byte(`{"msg":"tampered"}`)

	res := VerifyChain([]ChainRecord{r1, r2})
	if res.OK {
		t.Fatalf("expected failure, got ok")
	}
	if res.Failure == nil || res.Failure.Code != VerifyHashMismatch || res.Failure.ReceiptID != "r2" {
		t.Fatalf("unexpected failure: %+v", res.Failure)
	}
}

func TestVerifyChain_MissingLink(t *testing.T) {
	r1 := buildRecord(t, "r1", "", "one")
	r2 := buildRecord(t, "r2", r1.Hash, "two")
	r3 := buildRecord(t, "r3", r2.Hash, "three")

	res := VerifyChain([]ChainRecord{r1, r3})
	if res.OK {
		t.Fatalf("expected failure, got ok")
	}
	if res.Failure == nil || res.Failure.Code != VerifyMissingLink || res.Failure.ReceiptID != "r3" {
		t.Fatalf("unexpected failure: %+v", res.Failure)
	}
}

func TestVerifyChain_OutOfOrder(t *testing.T) {
	r1 := buildRecord(t, "r1", "", "one")
	r2 := buildRecord(t, "r2", r1.Hash, "two")
	r3 := buildRecord(t, "r3", r2.Hash, "three")

	res := VerifyChain([]ChainRecord{r1, r3, r2})
	if res.OK {
		t.Fatalf("expected failure, got ok")
	}
	if res.Failure == nil || res.Failure.Code != VerifyOutOfOrder || res.Failure.ReceiptID != "r3" {
		t.Fatalf("unexpected failure: %+v", res.Failure)
	}
}
