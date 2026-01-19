package testutil

import (
	"testing"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
)

type ReceiptIngestResponse struct {
	ReceiptID string  `json:"receipt_id"`
	Hash      string  `json:"hash"`
	PrevHash  *string `json:"prev_hash,omitempty"`
}

type PEPAllowResponse struct {
	Status string `json:"status"`
}

func AssertErrorEnvelope(t *testing.T, resp protocol.ErrorResponse, expectedCode, expectedMessage, expectedRequestID, expectedDecisionID, expectedTraceID string) {
	t.Helper()
	if expectedCode != "" && resp.Error.Code != expectedCode {
		t.Fatalf("expected error.code %q, got %q", expectedCode, resp.Error.Code)
	}
	if expectedMessage != "" && resp.Error.Message != expectedMessage {
		t.Fatalf("expected error.message %q, got %q", expectedMessage, resp.Error.Message)
	}
	if expectedRequestID != "" {
		AssertStringMatch(t, expectedRequestID, resp.RequestID, "request_id")
	}
	if expectedDecisionID != "" {
		AssertStringMatch(t, expectedDecisionID, resp.DecisionID, "decision_id")
	}
	if expectedTraceID != "" {
		AssertStringMatch(t, expectedTraceID, resp.TraceID, "trace_id")
	}
}

func AssertDecisionResponse(t *testing.T, expected, actual protocol.DecisionResponse) {
	t.Helper()
	if expected.Decision != "" && actual.Decision != expected.Decision {
		t.Fatalf("decision mismatch: expected %q got %q", expected.Decision, actual.Decision)
	}
	if expected.DecisionID != "" {
		AssertStringMatch(t, expected.DecisionID, actual.DecisionID, "decision_id")
	}
	if expected.Reason != "" && actual.Reason != expected.Reason {
		t.Fatalf("reason mismatch: expected %q got %q", expected.Reason, actual.Reason)
	}
	if expected.PolicyHash != "" && actual.PolicyHash != expected.PolicyHash {
		t.Fatalf("policy_hash mismatch: expected %q got %q", expected.PolicyHash, actual.PolicyHash)
	}
	if expected.PolicyVersion != 0 && actual.PolicyVersion != expected.PolicyVersion {
		t.Fatalf("policy_version mismatch: expected %d got %d", expected.PolicyVersion, actual.PolicyVersion)
	}
	if expected.RequestID != "" {
		AssertStringMatch(t, expected.RequestID, actual.RequestID, "request_id")
	}
	if expected.TraceID != "" {
		AssertStringMatch(t, expected.TraceID, actual.TraceID, "trace_id")
	}
	if expected.SpanID != "" {
		AssertStringMatch(t, expected.SpanID, actual.SpanID, "span_id")
	}
}

func AssertReceiptIngestResponse(t *testing.T, expected, actual ReceiptIngestResponse) {
	t.Helper()
	AssertStringMatch(t, expected.ReceiptID, actual.ReceiptID, "receipt_id")
	AssertStringMatch(t, expected.Hash, actual.Hash, "hash")
	if expected.PrevHash == nil {
		if actual.PrevHash != nil {
			t.Fatalf("unexpected prev_hash")
		}
		return
	}
	if actual.PrevHash == nil {
		t.Fatalf("expected prev_hash to be set")
	}
	AssertStringMatch(t, *expected.PrevHash, *actual.PrevHash, "prev_hash")
}

func AssertPEPAllowResponse(t *testing.T, expected, actual PEPAllowResponse) {
	t.Helper()
	if expected.Status != "" && actual.Status != expected.Status {
		t.Fatalf("status mismatch: expected %q got %q", expected.Status, actual.Status)
	}
}
