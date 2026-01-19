package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
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

func (s *Store) FindInvocationReceiptByRequestID(ctx context.Context, tenant uuid.UUID, requestID string, since time.Time) ([]byte, bool, error) {
	var body []byte
	err := s.db.QueryRow(ctx, `
    SELECT body_canonical
    FROM receipts_invocation
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&body)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return body, true, nil
}

func (s *Store) InsertInvocationReceiptIdempotent(
	ctx context.Context,
	tenant uuid.UUID,
	requestID string,
	decisionID *uuid.UUID,
	toolName string,
	method string,
	path string,
	outcome string,
	statusCode *int,
	latencyMs int,
	body json.RawMessage,
	traceID string,
	spanID string,
	since time.Time,
	chainScope string,
) (receipts.IdempotencyOutcome, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return receipts.IdempotencyConflict, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if strings.TrimSpace(requestID) != "" {
		lockKey1, lockKey2 := receipts.AdvisoryLockPair(tenant.String(), "invocation", requestID)
		if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
			return receipts.IdempotencyConflict, err
		}
		existing, ok, err := findInvocationReceiptByRequestIDTx(ctx, tx, tenant, requestID, since)
		if err != nil {
			return receipts.IdempotencyConflict, err
		}
		if ok {
			if len(existing) == 0 || !bytes.Equal(existing, body) {
				return receipts.IdempotencyConflict, nil
			}
			if err := tx.Commit(ctx); err != nil {
				return receipts.IdempotencyConflict, err
			}
			return receipts.IdempotencyReplayed, nil
		}
	}

	lockKey1, lockKey2 := receipts.ChainLockPair(tenant.String(), "invocation", time.Now().UTC(), chainScope)
	if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
		return receipts.IdempotencyConflict, err
	}

	prevHash, err := lastInvocationHashTx(ctx, tx, tenant)
	if err != nil {
		return receipts.IdempotencyConflict, err
	}
	hash := receipts.HashBytes(append([]byte(prevHash), body...))

	_, err = tx.Exec(ctx, `
    INSERT INTO receipts_invocation(tenant_id, decision_id, request_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenant,
		decisionID,
		nullIfEmpty(requestID),
		toolName,
		method,
		path,
		outcome,
		statusCode,
		latencyMs,
		body,
		body,
		nullIfEmpty(prevHash),
		hash,
		nullIfEmpty(traceID),
		nullIfEmpty(spanID),
	)
	if err != nil {
		return receipts.IdempotencyConflict, err
	}
	if err := tx.Commit(ctx); err != nil {
		return receipts.IdempotencyConflict, err
	}
	return receipts.IdempotencyInserted, nil
}

func (s *Store) InsertInvocationReceipt(ctx context.Context,
	tenant uuid.UUID,
	decisionID *uuid.UUID,
	requestID string,
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
    INSERT INTO receipts_invocation(tenant_id, decision_id, request_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenant,
		decisionID,
		nullIfEmpty(requestID),
		toolName,
		method,
		path,
		outcome,
		statusCode,
		latencyMs,
		body,
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

func findInvocationReceiptByRequestIDTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, requestID string, since time.Time) ([]byte, bool, error) {
	var body []byte
	err := tx.QueryRow(ctx, `
    SELECT body_canonical
    FROM receipts_invocation
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&body)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return body, true, nil
}

func lastInvocationHashTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) (string, error) {
	var h *string
	err := tx.QueryRow(ctx, `
    SELECT hash FROM receipts_invocation
    WHERE tenant_id=$1
    ORDER BY ts DESC
    LIMIT 1`, tenant).Scan(&h)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if h == nil {
		return "", nil
	}
	return *h, nil
}
