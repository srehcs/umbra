package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
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

func (s *Store) InsertDecisionReceipt(ctx context.Context,
	tenant uuid.UUID,
	decisionID uuid.UUID,
	policyHash string,
	decision string,
	body json.RawMessage,
	prevHash string,
	hash string,
	traceID string,
	spanID string,
) error {
	_, err := s.db.Exec(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, policy_hash, decision, body_json, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		tenant, decisionID, policyHash, decision, body, nullIfEmpty(prevHash), hash, nullIfEmpty(traceID), nullIfEmpty(spanID))
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

var _ = time.Now
