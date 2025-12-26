package storage

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
)

type Store struct{ db *pgxpool.Pool }

func New(db *stor.DB) *Store { return &Store{db: db.Pool} }

func (s *Store) LastInvocationHash(ctx context.Context, tenant uuid.UUID) (string, error) {
	var h *string
	err := s.db.QueryRow(ctx, `
    SELECT hash FROM receipts_invocation
    WHERE tenant_id=$1
    ORDER BY ts DESC
    LIMIT 1`, tenant).Scan(&h)
	if err != nil {
		return "", err
	}
	if h == nil {
		return "", nil
	}
	return *h, nil
}

func (s *Store) InsertInvocationReceipt(ctx context.Context,
	tenant uuid.UUID,
	decisionID *uuid.UUID,
	toolName string,
	method string,
	path string,
	outcome string,
	statusCode *int,
	latencyMs int,
	body json.RawMessage,
	prevHash string,
	hash string,
	traceID string,
	spanID string,
) error {
	_, err := s.db.Exec(ctx, `
    INSERT INTO receipts_invocation(tenant_id, decision_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenant,
		decisionID,
		toolName,
		method,
		path,
		outcome,
		statusCode,
		latencyMs,
		body,
		nullIfEmpty(prevHash),
		hash,
		nullIfEmpty(traceID),
		nullIfEmpty(spanID),
	)
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
