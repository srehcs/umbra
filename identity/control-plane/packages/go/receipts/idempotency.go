package receipts

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

// IdempotencyOutcome represents the result of an idempotent insert attempt.
type IdempotencyOutcome int

const (
	IdempotencyInserted IdempotencyOutcome = iota
	IdempotencyReplayed
	IdempotencyConflict
)

// DefaultRequestIDDedupeWindow is the default idempotency window for request_id.
const DefaultRequestIDDedupeWindow = 24 * time.Hour

// ParseRequestIDDedupeWindow parses a duration string for request_id dedupe windows.
func ParseRequestIDDedupeWindow(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("request_id dedupe window is empty")
	}
	window, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("request_id dedupe window invalid: %w", err)
	}
	if window <= 0 {
		return 0, fmt.Errorf("request_id dedupe window must be positive")
	}
	return window, nil
}

// RequestIDDedupeSince returns the cutoff timestamp for idempotency checks.
func RequestIDDedupeSince(now time.Time, window time.Duration) time.Time {
	return now.Add(-window)
}

// IdempotencyPayload captures the persisted fields used for idempotency matching.
type IdempotencyPayload struct {
	Kind       string          `json:"kind"`
	RequestID  string          `json:"request_id"`
	DecisionID string          `json:"decision_id,omitempty"`
	Decision   string          `json:"decision,omitempty"`
	PolicyHash string          `json:"policy_hash,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Method     string          `json:"method,omitempty"`
	Path       string          `json:"path,omitempty"`
	Outcome    string          `json:"outcome,omitempty"`
	Body       json.RawMessage `json:"body"`
}

// CanonicalizeIdempotencyPayload returns canonical JSON for idempotency matching.
func CanonicalizeIdempotencyPayload(payload IdempotencyPayload) ([]byte, error) {
	return CanonicalJSON(payload)
}

// AdvisoryLockPair returns a stable pair of 32-bit keys for advisory locks.
func AdvisoryLockPair(tenantID string, kind string, requestID string) (int32, int32) {
	return hash32(tenantID, kind), hash32(requestID)
}

// ResolveRequestIDDedupeWindow returns the configured window or the default.
func ResolveRequestIDDedupeWindow(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultRequestIDDedupeWindow, nil
	}
	window, err := ParseRequestIDDedupeWindow(raw)
	if err != nil {
		return DefaultRequestIDDedupeWindow, err
	}
	return window, nil
}

// ResolveChainLockScope validates the chain lock scope.
func ResolveChainLockScope(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "tenant", nil
	}
	switch value {
	case "tenant", "day":
		return value, nil
	default:
		return "tenant", fmt.Errorf("invalid chain lock scope: %s", value)
	}
}

// ChainLockPair returns the advisory lock keys for chain serialization.
func ChainLockPair(tenantID string, kind string, now time.Time, scope string) (int32, int32) {
	switch scope {
	case "day":
		return AdvisoryLockPair(tenantID, kind, "chain:"+now.UTC().Format("2006-01-02"))
	default:
		return AdvisoryLockPair(tenantID, kind, "chain")
	}
}

func hash32(parts ...string) int32 {
	h := fnv.New32a()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return int32(h.Sum32())
}
