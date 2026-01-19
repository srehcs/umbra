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

type ActivePolicy struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	Name       string
	Version    int
	Policy     json.RawMessage
	PolicyHash string
}

func (s *Store) GetActivePolicy(ctx context.Context, tenant uuid.UUID) (ActivePolicy, error) {
	var ap ActivePolicy
	err := s.db.QueryRow(ctx, `
    SELECT id, tenant_id, name, version, policy_json, policy_hash
    FROM policies
    WHERE tenant_id=$1 AND active=true
    ORDER BY updated_at DESC
    LIMIT 1`, tenant).
		Scan(&ap.ID, &ap.TenantID, &ap.Name, &ap.Version, &ap.Policy, &ap.PolicyHash)
	return ap, err
}

func (s *Store) LastDecisionHash(ctx context.Context, tenant uuid.UUID) (string, error) {
	var h *string
	err := s.db.QueryRow(ctx, `
    SELECT hash FROM receipts_decision
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

func (s *Store) FindDecisionReceiptByRequestID(ctx context.Context, tenant uuid.UUID, requestID string, since time.Time) ([]byte, bool, error) {
	var body []byte
	err := s.db.QueryRow(ctx, `
    SELECT body_canonical
    FROM receipts_decision
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

func (s *Store) InsertDecisionReceiptIdempotent(
	ctx context.Context,
	tenant uuid.UUID,
	requestID string,
	decisionID uuid.UUID,
	policyHash string,
	decision string,
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
		lockKey1, lockKey2 := receipts.AdvisoryLockPair(tenant.String(), "decision", requestID)
		if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
			return receipts.IdempotencyConflict, err
		}
		existing, ok, err := s.findDecisionReceiptByRequestIDTx(ctx, tx, tenant, requestID, since)
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

	lockKey1, lockKey2 := receipts.ChainLockPair(tenant.String(), "decision", time.Now().UTC(), chainScope)
	if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
		return receipts.IdempotencyConflict, err
	}

	prevHash, err := lastDecisionHashTx(ctx, tx, tenant)
	if err != nil {
		return receipts.IdempotencyConflict, err
	}
	hash := receipts.HashBytes(append([]byte(prevHash), body...))

	_, err = tx.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, request_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenant, decisionID, nullIfEmpty(requestID), policyHash, decision, body, body, nullIfEmpty(prevHash), hash, nullIfEmpty(traceID), nullIfEmpty(spanID))
	if err != nil {
		return receipts.IdempotencyConflict, err
	}
	if err := tx.Commit(ctx); err != nil {
		return receipts.IdempotencyConflict, err
	}
	return receipts.IdempotencyInserted, nil
}

func (s *Store) InsertDecisionReceipt(ctx context.Context,
	tenant uuid.UUID,
	decisionID uuid.UUID,
	requestID string,
	policyHash string,
	decision string,
	body json.RawMessage,
	prevHash string,
	hash string,
	traceID string,
	spanID string,
) error {
	_, err := s.db.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, request_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenant, decisionID, nullIfEmpty(requestID), policyHash, decision, body, body, nullIfEmpty(prevHash), hash, nullIfEmpty(traceID), nullIfEmpty(spanID))
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) findDecisionReceiptByRequestIDTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, requestID string, since time.Time) ([]byte, bool, error) {
	var body []byte
	err := tx.QueryRow(ctx, `
    SELECT body_canonical
    FROM receipts_decision
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

func lastDecisionHashTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) (string, error) {
	var h *string
	err := tx.QueryRow(ctx, `
    SELECT hash FROM receipts_decision
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

var _ = time.Now
