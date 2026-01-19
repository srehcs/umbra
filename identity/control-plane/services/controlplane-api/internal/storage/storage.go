package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
)

type Store struct{ db *pgxpool.Pool }

func New(db *stor.DB) *Store { return &Store{db: db.Pool} }

type Tool struct {
	ID        uuid.UUID       `json:"id"`
	TenantID  uuid.UUID       `json:"tenant_id"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Config    json.RawMessage `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Policy struct {
	ID         uuid.UUID       `json:"id"`
	TenantID   uuid.UUID       `json:"tenant_id"`
	Name       string          `json:"name"`
	Version    int             `json:"version"`
	Active     bool            `json:"active"`
	Policy     json.RawMessage `json:"policy"`
	PolicyHash string          `json:"policy_hash"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func (s *Store) ListTools(ctx context.Context, tenant uuid.UUID, limit int) ([]Tool, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
    SELECT id, tenant_id, name, kind, config_json, created_at, updated_at
    FROM tools
    WHERE tenant_id=$1
    ORDER BY created_at DESC
    LIMIT $2`, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Tool{}
	for rows.Next() {
		var t Tool
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Kind, &t.Config, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CreateTool(ctx context.Context, tenant uuid.UUID, name, kind string, config json.RawMessage) (Tool, error) {
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	var t Tool
	err := s.db.QueryRow(ctx, `
    INSERT INTO tools(tenant_id, name, kind, config_json)
    VALUES ($1,$2,$3,$4)
    RETURNING id, tenant_id, name, kind, config_json, created_at, updated_at`,
		tenant, name, kind, config).Scan(&t.ID, &t.TenantID, &t.Name, &t.Kind, &t.Config, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (s *Store) ListPolicies(ctx context.Context, tenant uuid.UUID, limit int) ([]Policy, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
    SELECT id, tenant_id, name, version, active, policy_json, policy_hash, created_at, updated_at
    FROM policies
    WHERE tenant_id=$1
    ORDER BY updated_at DESC
    LIMIT $2`, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Policy{}
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Version, &p.Active, &p.Policy, &p.PolicyHash, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreatePolicy(ctx context.Context, tenant uuid.UUID, name string, policy json.RawMessage, policyHash string) (Policy, error) {
	var p Policy
	err := s.db.QueryRow(ctx, `
    INSERT INTO policies(tenant_id, name, version, active, policy_json, policy_hash)
    VALUES ($1,$2,1,false,$3,$4)
    RETURNING id, tenant_id, name, version, active, policy_json, policy_hash, created_at, updated_at`,
		tenant, name, policy, policyHash).Scan(&p.ID, &p.TenantID, &p.Name, &p.Version, &p.Active, &p.Policy, &p.PolicyHash, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) GetPolicy(ctx context.Context, tenant, policyID uuid.UUID) (Policy, error) {
	var p Policy
	err := s.db.QueryRow(ctx, `
    SELECT id, tenant_id, name, version, active, policy_json, policy_hash, created_at, updated_at
    FROM policies
    WHERE tenant_id=$1 AND id=$2`,
		tenant, policyID).Scan(&p.ID, &p.TenantID, &p.Name, &p.Version, &p.Active, &p.Policy, &p.PolicyHash, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) UpdatePolicy(ctx context.Context, tenant, policyID uuid.UUID, policy json.RawMessage, policyHash string) (Policy, error) {
	var p Policy
	err := s.db.QueryRow(ctx, `
    UPDATE policies
    SET policy_json=$3, policy_hash=$4, version=version+1, updated_at=now()
    WHERE tenant_id=$1 AND id=$2 AND active=false
    RETURNING id, tenant_id, name, version, active, policy_json, policy_hash, created_at, updated_at`,
		tenant, policyID, policy, policyHash).Scan(&p.ID, &p.TenantID, &p.Name, &p.Version, &p.Active, &p.Policy, &p.PolicyHash, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) ActivatePolicy(ctx context.Context, tenant, policyID uuid.UUID) error {
	// deactivate others, activate this one
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE policies SET active=false WHERE tenant_id=$1`, tenant); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE policies SET active=true, updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenant, policyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return stor.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) GetActivePolicy(ctx context.Context, tenant uuid.UUID) (Policy, error) {
	var p Policy
	err := s.db.QueryRow(ctx, `
    SELECT id, tenant_id, name, version, active, policy_json, policy_hash, created_at, updated_at
    FROM policies
    WHERE tenant_id=$1 AND active=true
    ORDER BY updated_at DESC
    LIMIT 1`, tenant).Scan(&p.ID, &p.TenantID, &p.Name, &p.Version, &p.Active, &p.Policy, &p.PolicyHash, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

type Receipt struct {
	Kind string          `json:"kind"` // decision|invocation
	TS   time.Time       `json:"ts"`
	Data json.RawMessage `json:"data"`
}

type ExportFilters struct {
	From       *time.Time
	To         *time.Time
	ActorID    string
	Tool       string
	Decision   string
	RequestID  string
	DecisionID string
	Limit      int
}

type ExportRecord struct {
	SchemaVersion   string    `json:"schema_version"`
	Kind            string    `json:"kind"`
	TS              time.Time `json:"ts"`
	RequestID       string    `json:"request_id,omitempty"`
	DecisionID      string    `json:"decision_id,omitempty"`
	TraceID         string    `json:"trace_id,omitempty"`
	PolicyHash      string    `json:"policy_hash,omitempty"`
	PolicyVersion   *int      `json:"policy_version,omitempty"`
	Decision        string    `json:"decision,omitempty"`
	ActorID         string    `json:"actor_id,omitempty"`
	ToolName        string    `json:"tool_name,omitempty"`
	Method          string    `json:"method,omitempty"`
	Path            string    `json:"path,omitempty"`
	Outcome         string    `json:"outcome,omitempty"`
	StatusCode      *int      `json:"status_code,omitempty"`
	ReceiptHash     string    `json:"receipt_hash"`
	ReceiptPrevHash string    `json:"receipt_prev_hash,omitempty"`
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

type ReceiptIdempotencyRecord struct {
	ID            uuid.UUID
	Hash          string
	PrevHash      string
	BodyCanonical []byte
	Payload       receipts.IdempotencyPayload
}

func (s *Store) FindDecisionReceiptByRequestID(ctx context.Context, tenant uuid.UUID, requestID string, since time.Time) (ReceiptIdempotencyRecord, bool, error) {
	var rec ReceiptIdempotencyRecord
	var prev *string
	err := s.db.QueryRow(ctx, `
    SELECT id, hash, prev_hash, body_canonical, decision_id, policy_hash, decision, request_id
    FROM receipts_decision
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&rec.ID, &rec.Hash, &prev, &rec.BodyCanonical, &rec.Payload.DecisionID, &rec.Payload.PolicyHash, &rec.Payload.Decision, &rec.Payload.RequestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, false, nil
		}
		return rec, false, err
	}
	if prev != nil {
		rec.PrevHash = *prev
	}
	rec.Payload.Kind = "decision"
	rec.Payload.Body = json.RawMessage(rec.BodyCanonical)
	return rec, true, nil
}

func (s *Store) FindInvocationReceiptByRequestID(ctx context.Context, tenant uuid.UUID, requestID string, since time.Time) (ReceiptIdempotencyRecord, bool, error) {
	var rec ReceiptIdempotencyRecord
	var prev *string
	var decisionID sql.NullString
	err := s.db.QueryRow(ctx, `
    SELECT id, hash, prev_hash, body_canonical, decision_id, tool_name, method, path, outcome, request_id
    FROM receipts_invocation
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&rec.ID, &rec.Hash, &prev, &rec.BodyCanonical, &decisionID, &rec.Payload.ToolName, &rec.Payload.Method, &rec.Payload.Path, &rec.Payload.Outcome, &rec.Payload.RequestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, false, nil
		}
		return rec, false, err
	}
	if prev != nil {
		rec.PrevHash = *prev
	}
	rec.Payload.Kind = "invocation"
	if decisionID.Valid {
		rec.Payload.DecisionID = decisionID.String
	}
	rec.Payload.Body = json.RawMessage(rec.BodyCanonical)
	return rec, true, nil
}

type IdempotentInsertResult struct {
	Outcome receipts.IdempotencyOutcome
	Record  ReceiptIdempotencyRecord
}

func (s *Store) InsertDecisionReceiptIdempotent(
	ctx context.Context,
	tenant uuid.UUID,
	requestID string,
	decisionID uuid.UUID,
	policyHash string,
	decision string,
	body json.RawMessage,
	idemBody json.RawMessage,
	traceID string,
	spanID string,
	since time.Time,
	chainScope string,
) (IdempotentInsertResult, error) {
	return s.insertReceiptIdempotent(ctx, tenant, "decision", requestID, since, idemBody, chainScope, func(tx pgx.Tx) (ReceiptIdempotencyRecord, error) {
		prev, err := lastDecisionHashTx(ctx, tx, tenant)
		if err != nil {
			return ReceiptIdempotencyRecord{}, err
		}
		hash := receipts.HashBytes(append([]byte(prev), body...))
		var id uuid.UUID
		err = tx.QueryRow(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, request_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    RETURNING id`,
			tenant, decisionID, nullIfEmpty(requestID), policyHash, decision, body, body, nullIfEmpty(prev), hash, nullIfEmpty(traceID), nullIfEmpty(spanID),
		).Scan(&id)
		if err != nil {
			return ReceiptIdempotencyRecord{}, err
		}
		return ReceiptIdempotencyRecord{ID: id, Hash: hash, PrevHash: prev, BodyCanonical: body}, nil
	})
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
	idemBody json.RawMessage,
	traceID string,
	spanID string,
	since time.Time,
	chainScope string,
) (IdempotentInsertResult, error) {
	return s.insertReceiptIdempotent(ctx, tenant, "invocation", requestID, since, idemBody, chainScope, func(tx pgx.Tx) (ReceiptIdempotencyRecord, error) {
		prev, err := lastInvocationHashTx(ctx, tx, tenant)
		if err != nil {
			return ReceiptIdempotencyRecord{}, err
		}
		hash := receipts.HashBytes(append([]byte(prev), body...))
		var id uuid.UUID
		err = tx.QueryRow(ctx, `
    INSERT INTO receipts_invocation(tenant_id, decision_id, request_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    RETURNING id`,
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
			nullIfEmpty(prev),
			hash,
			nullIfEmpty(traceID),
			nullIfEmpty(spanID),
		).Scan(&id)
		if err != nil {
			return ReceiptIdempotencyRecord{}, err
		}
		return ReceiptIdempotencyRecord{ID: id, Hash: hash, PrevHash: prev, BodyCanonical: body}, nil
	})
}

func (s *Store) insertReceiptIdempotent(
	ctx context.Context,
	tenant uuid.UUID,
	kind string,
	requestID string,
	since time.Time,
	idemBody json.RawMessage,
	chainScope string,
	insert func(pgx.Tx) (ReceiptIdempotencyRecord, error),
) (IdempotentInsertResult, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return IdempotentInsertResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if strings.TrimSpace(requestID) != "" {
		lockKey1, lockKey2 := receipts.AdvisoryLockPair(tenant.String(), kind, requestID)
		if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
			return IdempotentInsertResult{}, err
		}

		var existing ReceiptIdempotencyRecord
		var ok bool
		switch kind {
		case "decision":
			existing, ok, err = findDecisionReceiptByRequestIDTx(ctx, tx, tenant, requestID, since)
		case "invocation":
			existing, ok, err = findInvocationReceiptByRequestIDTx(ctx, tx, tenant, requestID, since)
		}
		if err != nil {
			return IdempotentInsertResult{}, err
		}
		if ok {
			existingBytes, err := receipts.CanonicalizeIdempotencyPayload(existing.Payload)
			if err != nil {
				return IdempotentInsertResult{}, err
			}
			if len(existingBytes) == 0 || !bytes.Equal(existingBytes, idemBody) {
				return IdempotentInsertResult{Outcome: receipts.IdempotencyConflict}, nil
			}
			if err := tx.Commit(ctx); err != nil {
				return IdempotentInsertResult{}, err
			}
			return IdempotentInsertResult{Outcome: receipts.IdempotencyReplayed, Record: existing}, nil
		}
	}

	lockKey1, lockKey2 := receipts.ChainLockPair(tenant.String(), kind, time.Now().UTC(), chainScope)
	if err := stor.AdvisoryLockTx(ctx, tx, lockKey1, lockKey2); err != nil {
		return IdempotentInsertResult{}, err
	}

	record, err := insert(tx)
	if err != nil {
		return IdempotentInsertResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return IdempotentInsertResult{}, err
	}
	return IdempotentInsertResult{Outcome: receipts.IdempotencyInserted, Record: record}, nil
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
) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
    INSERT INTO receipts_decision(tenant_id, decision_id, request_id, policy_hash, decision, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    RETURNING id`,
		tenant, decisionID, nullIfEmpty(requestID), policyHash, decision, body, body, nullIfEmpty(prevHash), hash, nullIfEmpty(traceID), nullIfEmpty(spanID)).
		Scan(&id)
	return id, err
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
) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
    INSERT INTO receipts_invocation(tenant_id, decision_id, request_id, tool_name, method, path, outcome, status_code, latency_ms, body_json, body_canonical, prev_hash, hash, trace_id, span_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
    RETURNING id`,
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
	).Scan(&id)
	return id, err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func lastDecisionHashTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) (string, error) {
	var h *string
	if err := tx.QueryRow(ctx, `
    SELECT hash FROM receipts_decision
    WHERE tenant_id=$1
    ORDER BY ts DESC
    LIMIT 1`, tenant).Scan(&h); err != nil {
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

func lastInvocationHashTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) (string, error) {
	var h *string
	if err := tx.QueryRow(ctx, `
    SELECT hash FROM receipts_invocation
    WHERE tenant_id=$1
    ORDER BY ts DESC
    LIMIT 1`, tenant).Scan(&h); err != nil {
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

func findDecisionReceiptByRequestIDTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, requestID string, since time.Time) (ReceiptIdempotencyRecord, bool, error) {
	var rec ReceiptIdempotencyRecord
	var prev *string
	err := tx.QueryRow(ctx, `
    SELECT id, hash, prev_hash, body_canonical, decision_id, policy_hash, decision, request_id
    FROM receipts_decision
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&rec.ID, &rec.Hash, &prev, &rec.BodyCanonical, &rec.Payload.DecisionID, &rec.Payload.PolicyHash, &rec.Payload.Decision, &rec.Payload.RequestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, false, nil
		}
		return rec, false, err
	}
	if prev != nil {
		rec.PrevHash = *prev
	}
	rec.Payload.Kind = "decision"
	rec.Payload.Body = json.RawMessage(rec.BodyCanonical)
	return rec, true, nil
}

func findInvocationReceiptByRequestIDTx(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, requestID string, since time.Time) (ReceiptIdempotencyRecord, bool, error) {
	var rec ReceiptIdempotencyRecord
	var prev *string
	var decisionID sql.NullString
	err := tx.QueryRow(ctx, `
    SELECT id, hash, prev_hash, body_canonical, decision_id, tool_name, method, path, outcome, request_id
    FROM receipts_invocation
    WHERE tenant_id=$1 AND request_id=$2 AND ts >= $3
    ORDER BY ts DESC
    LIMIT 1`, tenant, requestID, since).Scan(&rec.ID, &rec.Hash, &prev, &rec.BodyCanonical, &decisionID, &rec.Payload.ToolName, &rec.Payload.Method, &rec.Payload.Path, &rec.Payload.Outcome, &rec.Payload.RequestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, false, nil
		}
		return rec, false, err
	}
	if prev != nil {
		rec.PrevHash = *prev
	}
	rec.Payload.Kind = "invocation"
	if decisionID.Valid {
		rec.Payload.DecisionID = decisionID.String
	}
	rec.Payload.Body = json.RawMessage(rec.BodyCanonical)
	return rec, true, nil
}

func (s *Store) ListReceiptChain(ctx context.Context, tenant uuid.UUID, limit int, kind string) ([]receipts.ChainRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "decision":
		return s.listReceiptChain(ctx, tenant, limit, "receipts_decision")
	case "invocation":
		return s.listReceiptChain(ctx, tenant, limit, "receipts_invocation")
	default:
		return nil, stor.ErrNotFound
	}
}

func (s *Store) ListReceipts(ctx context.Context, tenant uuid.UUID, limit int, kind string, q string, before *time.Time) ([]json.RawMessage, *time.Time, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	q = strings.ToLower(strings.TrimSpace(q))

	if kind == "" || kind == "all" {
		return s.listReceiptsUnion(ctx, tenant, limit, q, before)
	}
	if kind == "decision" {
		return s.listReceiptsDecision(ctx, tenant, limit, q, before)
	}
	if kind == "invocation" {
		return s.listReceiptsInvocation(ctx, tenant, limit, q, before)
	}

	return []json.RawMessage{}, nil, nil
}

func (s *Store) listReceiptsUnion(ctx context.Context, tenant uuid.UUID, limit int, q string, before *time.Time) ([]json.RawMessage, *time.Time, error) {
	args := []interface{}{tenant}
	idx := 2
	whereDecision := " WHERE tenant_id=$1"
	whereInvocation := " WHERE tenant_id=$1"
	if before != nil {
		whereDecision += " AND ts < $" + itoa(idx)
		whereInvocation += " AND ts < $" + itoa(idx)
		args = append(args, *before)
		idx++
	}

	query := `
    SELECT ts, obj, id, request_id, decision_id, trace_id, receipt_hash, receipt_prev_hash, search_text FROM (
      SELECT ts, jsonb_build_object(
        'kind','decision',
        'id', id,
        'ts', ts,
        'decision_id', decision_id,
        'request_id', request_id,
        'policy_hash', policy_hash,
        'decision', decision,
        'hash', hash,
        'prev_hash', prev_hash,
        'trace_id', trace_id,
        'span_id', span_id
      ) AS obj,
      id,
      request_id,
      decision_id::text AS decision_id,
      trace_id,
      hash AS receipt_hash,
      prev_hash AS receipt_prev_hash,
      search_text
      FROM receipts_decision` + whereDecision + `
      UNION ALL
      SELECT ts, jsonb_build_object(
        'kind','invocation',
        'id', id,
        'ts', ts,
        'decision_id', decision_id,
        'request_id', request_id,
        'tool_name', tool_name,
        'method', method,
        'path', path,
        'outcome', outcome,
        'status_code', status_code,
        'latency_ms', latency_ms,
        'policy_hash', body_json->>'policy_hash',
        'policy_version', (body_json->>'policy_version')::int,
        'hash', hash,
        'prev_hash', prev_hash,
        'trace_id', trace_id,
        'span_id', span_id
      ) AS obj,
      id,
      request_id,
      decision_id::text AS decision_id,
      trace_id,
      hash AS receipt_hash,
      prev_hash AS receipt_prev_hash,
      search_text
      FROM receipts_invocation` + whereInvocation + `
    ) AS merged`

	if q != "" {
		clause, clauseArgs, next := buildReceiptSearchClause(q, idx)
		query += clause
		args = append(args, clauseArgs...)
		idx = next
	}

	query += " ORDER BY ts DESC LIMIT $" + itoa(idx)
	args = append(args, limit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := []json.RawMessage{}
	var next *time.Time
	for rows.Next() {
		var ts time.Time
		var obj []byte
		var receiptID, requestID, decisionID, traceID, receiptHash string
		var receiptPrevHash *string
		var searchText *string
		if err := rows.Scan(&ts, &obj, &receiptID, &requestID, &decisionID, &traceID, &receiptHash, &receiptPrevHash, &searchText); err != nil {
			return nil, nil, err
		}
		_ = receiptID
		items = append(items, obj)
		next = &ts
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, next, nil
}

func (s *Store) listReceiptsDecision(ctx context.Context, tenant uuid.UUID, limit int, q string, before *time.Time) ([]json.RawMessage, *time.Time, error) {
	where, args := buildReceiptWhere(tenant, "receipts_decision", q, before, limit)
	query := `
    SELECT ts, jsonb_build_object(
      'kind','decision',
      'id', id,
      'ts', ts,
      'decision_id', decision_id,
      'request_id', request_id,
      'policy_hash', policy_hash,
      'decision', decision,
      'hash', hash,
      'prev_hash', prev_hash,
      'trace_id', trace_id,
      'span_id', span_id
    ) AS obj
    FROM receipts_decision` + where + `
    ORDER BY ts DESC
    LIMIT $` + itoa(len(args))

	return s.listReceiptRows(ctx, query, args)
}

func (s *Store) listReceiptsInvocation(ctx context.Context, tenant uuid.UUID, limit int, q string, before *time.Time) ([]json.RawMessage, *time.Time, error) {
	where, args := buildReceiptWhere(tenant, "receipts_invocation", q, before, limit)
	query := `
    SELECT ts, jsonb_build_object(
      'kind','invocation',
      'id', id,
      'ts', ts,
      'decision_id', decision_id,
      'request_id', request_id,
      'tool_name', tool_name,
      'method', method,
      'path', path,
      'outcome', outcome,
      'status_code', status_code,
      'latency_ms', latency_ms,
      'policy_hash', body_json->>'policy_hash',
      'policy_version', (body_json->>'policy_version')::int,
      'hash', hash,
      'prev_hash', prev_hash,
      'trace_id', trace_id,
      'span_id', span_id
    ) AS obj
    FROM receipts_invocation` + where + `
    ORDER BY ts DESC
    LIMIT $` + itoa(len(args))

	return s.listReceiptRows(ctx, query, args)
}

func (s *Store) listReceiptRows(ctx context.Context, query string, args []interface{}) ([]json.RawMessage, *time.Time, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := []json.RawMessage{}
	var next *time.Time
	for rows.Next() {
		var ts time.Time
		var obj []byte
		if err := rows.Scan(&ts, &obj); err != nil {
			return nil, nil, err
		}
		items = append(items, obj)
		next = &ts
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, next, nil
}

func buildReceiptWhere(tenant uuid.UUID, table string, q string, before *time.Time, limit int) (string, []interface{}) {
	args := []interface{}{tenant}
	idx := 2
	where := " WHERE tenant_id=$1"
	if before != nil {
		where += " AND ts < $" + itoa(idx)
		args = append(args, *before)
		idx++
	}
	if q != "" {
		if isUUID(q) {
			where += " AND (id::text = $" + itoa(idx) + " OR request_id = $" + itoa(idx) + " OR decision_id::text = $" + itoa(idx) + " OR trace_id = $" + itoa(idx) + ")"
			args = append(args, q)
			idx++
		} else if isHash(q) {
			where += " AND (hash = $" + itoa(idx) + " OR prev_hash = $" + itoa(idx) + ")"
			args = append(args, q)
			idx++
		} else {
			where += " AND search_text ILIKE $" + itoa(idx)
			args = append(args, "%"+q+"%")
			idx++
		}
	}
	args = append(args, limit)
	return where, args
}

func buildReceiptSearchClause(q string, start int) (string, []interface{}, int) {
	if q == "" {
		return "", nil, start
	}
	if isUUID(q) {
		return " WHERE (id::text = $" + itoa(start) + " OR request_id = $" + itoa(start) + " OR decision_id = $" + itoa(start) + " OR trace_id = $" + itoa(start) + ")",
			[]interface{}{q}, start + 1
	}
	if isHash(q) {
		return " WHERE (receipt_hash = $" + itoa(start) + " OR receipt_prev_hash = $" + itoa(start) + ")",
			[]interface{}{q}, start + 1
	}
	return " WHERE search_text ILIKE $" + itoa(start),
		[]interface{}{"%" + q + "%"}, start + 1
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func isHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func (s *Store) ExportReceiptsCSV(ctx context.Context, tenant uuid.UUID, f ExportFilters, w io.Writer) error {
	writer := csv.NewWriter(w)
	WriteReceiptsCSVHeader(writer)

	if f.ActorID != "" {
		rows, err := s.exportDecisionReceiptsRows(ctx, tenant, f)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			record, err := scanDecisionExportRow(rows)
			if err != nil {
				return err
			}
			WriteReceiptsCSVRecord(writer, record)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	}

	rows, err := s.exportReceiptsUnionRows(ctx, tenant, f)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		record, err := scanExportUnionRow(rows)
		if err != nil {
			return err
		}
		WriteReceiptsCSVRecord(writer, record)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func (s *Store) ExportReceiptsJSON(ctx context.Context, tenant uuid.UUID, f ExportFilters, w io.Writer) error {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	f.ActorID = strings.TrimSpace(f.ActorID)
	f.Tool = strings.TrimSpace(f.Tool)
	f.Decision = strings.ToLower(strings.TrimSpace(f.Decision))
	f.RequestID = strings.TrimSpace(f.RequestID)
	f.DecisionID = strings.TrimSpace(f.DecisionID)

	if f.ActorID != "" {
		rows, err := s.exportDecisionReceiptsRows(ctx, tenant, f)
		if err != nil {
			return err
		}
		defer rows.Close()
		return writeReceiptsJSONStream(rows, scanDecisionExportRow, w)
	}

	rows, err := s.exportReceiptsUnionRows(ctx, tenant, f)
	if err != nil {
		return err
	}
	defer rows.Close()
	return writeReceiptsJSONStream(rows, scanExportUnionRow, w)
}

func (s *Store) ExportReceipts(ctx context.Context, tenant uuid.UUID, f ExportFilters) ([]ExportRecord, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	f.ActorID = strings.TrimSpace(f.ActorID)
	f.Tool = strings.TrimSpace(f.Tool)
	f.Decision = strings.ToLower(strings.TrimSpace(f.Decision))
	f.RequestID = strings.TrimSpace(f.RequestID)
	f.DecisionID = strings.TrimSpace(f.DecisionID)

	if f.ActorID != "" {
		decisions, err := s.exportDecisionReceipts(ctx, tenant, f)
		if err != nil {
			return nil, err
		}
		return trimExport(decisions, f.Limit), nil
	}

	return s.exportReceiptsUnion(ctx, tenant, f)
}

func (s *Store) exportDecisionReceiptsRows(ctx context.Context, tenant uuid.UUID, f ExportFilters) (pgx.Rows, error) {
	args := []interface{}{tenant}
	idx := 2
	where := " WHERE tenant_id=$1 "
	if f.From != nil {
		where += " AND ts >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND ts <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.ActorID != "" {
		where += " AND body_json->'actor'->>'id' = $" + itoa(idx)
		args = append(args, f.ActorID)
		idx++
	}
	if f.Tool != "" {
		where += " AND body_json->'tool'->>'name' = $" + itoa(idx)
		args = append(args, f.Tool)
		idx++
	}
	if f.Decision != "" {
		where += " AND decision = $" + itoa(idx)
		args = append(args, f.Decision)
		idx++
	}
	if f.RequestID != "" {
		where += " AND request_id = $" + itoa(idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.DecisionID != "" {
		where += " AND decision_id = $" + itoa(idx)
		args = append(args, f.DecisionID)
		idx++
	}

	args = append(args, f.Limit)
	sql := `
    SELECT ts, decision_id, request_id, trace_id, policy_hash, decision, hash, prev_hash,
      body_json->'actor'->>'id' AS actor_id,
      body_json->'tool'->>'name' AS tool_name,
      NULLIF(body_json->>'policy_version','')::int AS policy_version
    FROM receipts_decision` + where + `
    ORDER BY ts DESC
    LIMIT $` + itoa(idx)

	return s.db.Query(ctx, sql, args...)
}

func (s *Store) exportDecisionReceipts(ctx context.Context, tenant uuid.UUID, f ExportFilters) ([]ExportRecord, error) {
	args := []interface{}{tenant}
	idx := 2
	where := " WHERE tenant_id=$1 "
	if f.From != nil {
		where += " AND ts >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND ts <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.ActorID != "" {
		where += " AND body_json->'actor'->>'id' = $" + itoa(idx)
		args = append(args, f.ActorID)
		idx++
	}
	if f.Tool != "" {
		where += " AND body_json->'tool'->>'name' = $" + itoa(idx)
		args = append(args, f.Tool)
		idx++
	}
	if f.Decision != "" {
		where += " AND decision = $" + itoa(idx)
		args = append(args, f.Decision)
		idx++
	}
	if f.RequestID != "" {
		where += " AND request_id = $" + itoa(idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.DecisionID != "" {
		where += " AND decision_id = $" + itoa(idx)
		args = append(args, f.DecisionID)
		idx++
	}

	args = append(args, f.Limit)
	sql := `
    SELECT ts, decision_id, request_id, trace_id, policy_hash, decision, hash, prev_hash,
      body_json->'actor'->>'id' AS actor_id,
      body_json->'tool'->>'name' AS tool_name,
      NULLIF(body_json->>'policy_version','')::int AS policy_version
    FROM receipts_decision` + where + `
    ORDER BY ts DESC
    LIMIT $` + itoa(idx)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ExportRecord{}
	for rows.Next() {
		record, err := scanDecisionExportRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) exportInvocationReceipts(ctx context.Context, tenant uuid.UUID, f ExportFilters) ([]ExportRecord, error) {
	args := []interface{}{tenant}
	idx := 2
	where := " WHERE tenant_id=$1 "
	if f.From != nil {
		where += " AND ts >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND ts <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.Tool != "" {
		where += " AND tool_name = $" + itoa(idx)
		args = append(args, f.Tool)
		idx++
	}
	if f.RequestID != "" {
		where += " AND request_id = $" + itoa(idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.DecisionID != "" {
		where += " AND decision_id = $" + itoa(idx)
		args = append(args, f.DecisionID)
		idx++
	}
	if f.Decision != "" {
		outcome := ""
		switch f.Decision {
		case "allow":
			outcome = "success"
		case "deny":
			outcome = "denied"
		default:
			outcome = ""
		}
		if outcome != "" {
			where += " AND outcome = $" + itoa(idx)
			args = append(args, outcome)
			idx++
		}
	}

	args = append(args, f.Limit)
	sql := `
    SELECT ts, decision_id, request_id, trace_id, tool_name, method, path, outcome, status_code, hash, prev_hash,
      body_json->>'policy_hash' AS policy_hash,
      NULLIF(body_json->>'policy_version','')::int AS policy_version
    FROM receipts_invocation` + where + `
    ORDER BY ts DESC
    LIMIT $` + itoa(idx)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ExportRecord{}
	for rows.Next() {
		var ts time.Time
		var decisionID *string
		var requestID, traceID, toolName, method, path, outcome string
		var statusCode *int
		var hash string
		var prev *string
		var policyHash *string
		var policyVersion *int
		if err := rows.Scan(&ts, &decisionID, &requestID, &traceID, &toolName, &method, &path, &outcome, &statusCode, &hash, &prev, &policyHash, &policyVersion); err != nil {
			return nil, err
		}
		record := ExportRecord{
			SchemaVersion:   "v1",
			Kind:            "invocation",
			TS:              ts,
			RequestID:       requestID,
			TraceID:         traceID,
			ToolName:        toolName,
			Method:          method,
			Path:            path,
			Outcome:         outcome,
			StatusCode:      statusCode,
			PolicyVersion:   policyVersion,
			ReceiptHash:     hash,
			ReceiptPrevHash: derefString(prev),
		}
		if decisionID != nil {
			record.DecisionID = *decisionID
		}
		if policyHash != nil {
			record.PolicyHash = *policyHash
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) exportReceiptsUnion(ctx context.Context, tenant uuid.UUID, f ExportFilters) ([]ExportRecord, error) {
	rows, err := s.exportReceiptsUnionRows(ctx, tenant, f)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ExportRecord{}
	for rows.Next() {
		record, err := scanExportUnionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) exportReceiptsUnionRows(ctx context.Context, tenant uuid.UUID, f ExportFilters) (pgx.Rows, error) {
	sql, args := buildExportUnionQuery(tenant, f)
	return s.db.Query(ctx, sql, args...)
}

func buildExportUnionQuery(tenant uuid.UUID, f ExportFilters) (string, []interface{}) {
	decisionWhere, decisionArgs, nextIdx := buildDecisionWhere(tenant, 1, f)
	invWhere, invArgs, nextIdx := buildInvocationWhere(tenant, nextIdx, f)

	decisionSQL := `
    SELECT
      'v1' AS schema_version,
      'decision' AS kind,
      ts,
      request_id,
      decision_id::text,
      trace_id,
      policy_hash,
      NULLIF(body_json->>'policy_version','')::int AS policy_version,
      decision,
      body_json->'actor'->>'id' AS actor_id,
      body_json->'tool'->>'name' AS tool_name,
      NULL::text AS method,
      NULL::text AS path,
      NULL::text AS outcome,
      NULL::int AS status_code,
      hash AS receipt_hash,
      prev_hash AS receipt_prev_hash
    FROM receipts_decision` + decisionWhere

	invSQL := `
    SELECT
      'v1' AS schema_version,
      'invocation' AS kind,
      ts,
      request_id,
      decision_id::text,
      trace_id,
      body_json->>'policy_hash' AS policy_hash,
      NULLIF(body_json->>'policy_version','')::int AS policy_version,
      NULL::text AS decision,
      NULL::text AS actor_id,
      tool_name,
      method,
      path,
      outcome,
      status_code,
      hash AS receipt_hash,
      prev_hash AS receipt_prev_hash
    FROM receipts_invocation` + invWhere

	args := append(decisionArgs, invArgs...)
	args = append(args, f.Limit)
	sql := decisionSQL + " UNION ALL " + invSQL + " ORDER BY ts DESC LIMIT $" + itoa(nextIdx)
	return sql, args
}

func scanExportUnionRow(rows pgx.Rows) (ExportRecord, error) {
	var schemaVersion, kind string
	var ts time.Time
	var requestID string
	var decisionID *string
	var traceID string
	var policyHash *string
	var policyVersion *int
	var decision *string
	var actorID *string
	var toolName *string
	var method *string
	var path *string
	var outcome *string
	var statusCode *int
	var receiptHash string
	var receiptPrevHash *string

	if err := rows.Scan(
		&schemaVersion,
		&kind,
		&ts,
		&requestID,
		&decisionID,
		&traceID,
		&policyHash,
		&policyVersion,
		&decision,
		&actorID,
		&toolName,
		&method,
		&path,
		&outcome,
		&statusCode,
		&receiptHash,
		&receiptPrevHash,
	); err != nil {
		return ExportRecord{}, err
	}

	record := ExportRecord{
		SchemaVersion:   schemaVersion,
		Kind:            kind,
		TS:              ts,
		RequestID:       requestID,
		TraceID:         traceID,
		PolicyVersion:   policyVersion,
		ReceiptHash:     receiptHash,
		ReceiptPrevHash: derefString(receiptPrevHash),
	}
	if decisionID != nil {
		record.DecisionID = *decisionID
	}
	if policyHash != nil {
		record.PolicyHash = *policyHash
	}
	if decision != nil {
		record.Decision = *decision
	}
	if actorID != nil {
		record.ActorID = *actorID
	}
	if toolName != nil {
		record.ToolName = *toolName
	}
	if method != nil {
		record.Method = *method
	}
	if path != nil {
		record.Path = *path
	}
	if outcome != nil {
		record.Outcome = *outcome
	}
	record.StatusCode = statusCode

	return record, nil
}

func scanDecisionExportRow(rows pgx.Rows) (ExportRecord, error) {
	var ts time.Time
	var decisionID, requestID, traceID, policyHash, decision string
	var hash string
	var prev *string
	var actorID, toolName *string
	var policyVersion *int
	if err := rows.Scan(&ts, &decisionID, &requestID, &traceID, &policyHash, &decision, &hash, &prev, &actorID, &toolName, &policyVersion); err != nil {
		return ExportRecord{}, err
	}
	record := ExportRecord{
		SchemaVersion:   "v1",
		Kind:            "decision",
		TS:              ts,
		RequestID:       requestID,
		DecisionID:      decisionID,
		TraceID:         traceID,
		PolicyHash:      policyHash,
		PolicyVersion:   policyVersion,
		Decision:        decision,
		ReceiptHash:     hash,
		ReceiptPrevHash: derefString(prev),
	}
	if actorID != nil {
		record.ActorID = *actorID
	}
	if toolName != nil {
		record.ToolName = *toolName
	}
	return record, nil
}

func writeReceiptsJSONStream(rows pgx.Rows, scan func(pgx.Rows) (ExportRecord, error), w io.Writer) error {
	if _, err := io.WriteString(w, `{"schema_version":"v1","items":[`); err != nil {
		return err
	}
	first := true
	for rows.Next() {
		record, err := scan(rows)
		if err != nil {
			return err
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if !first {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		first = false
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "]}")
	return err
}

func buildDecisionWhere(tenant uuid.UUID, start int, f ExportFilters) (string, []interface{}, int) {
	args := []interface{}{tenant}
	idx := start + 1
	where := " WHERE tenant_id=$" + itoa(start)
	if f.From != nil {
		where += " AND ts >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND ts <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.Tool != "" {
		where += " AND body_json->'tool'->>'name' = $" + itoa(idx)
		args = append(args, f.Tool)
		idx++
	}
	if f.Decision != "" {
		where += " AND decision = $" + itoa(idx)
		args = append(args, f.Decision)
		idx++
	}
	if f.RequestID != "" {
		where += " AND request_id = $" + itoa(idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.DecisionID != "" {
		where += " AND decision_id = $" + itoa(idx)
		args = append(args, f.DecisionID)
		idx++
	}
	return where, args, idx
}

func buildInvocationWhere(tenant uuid.UUID, start int, f ExportFilters) (string, []interface{}, int) {
	args := []interface{}{tenant}
	idx := start + 1
	where := " WHERE tenant_id=$" + itoa(start)
	if f.From != nil {
		where += " AND ts >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND ts <= $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}
	if f.Tool != "" {
		where += " AND tool_name = $" + itoa(idx)
		args = append(args, f.Tool)
		idx++
	}
	if f.RequestID != "" {
		where += " AND request_id = $" + itoa(idx)
		args = append(args, f.RequestID)
		idx++
	}
	if f.DecisionID != "" {
		where += " AND decision_id::text = $" + itoa(idx)
		args = append(args, f.DecisionID)
		idx++
	}
	if f.Decision != "" {
		outcome := ""
		switch f.Decision {
		case "allow":
			outcome = "success"
		case "deny":
			outcome = "denied"
		}
		if outcome != "" {
			where += " AND outcome = $" + itoa(idx)
			args = append(args, outcome)
			idx++
		}
	}
	return where, args, idx
}

func trimExport(items []ExportRecord, limit int) []ExportRecord {
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *Store) listReceiptChain(ctx context.Context, tenant uuid.UUID, limit int, table string) ([]receipts.ChainRecord, error) {
	rows, err := s.db.Query(ctx, `
    SELECT id, COALESCE(body_canonical, convert_to(body_json::text, 'utf8')) AS body, prev_hash, hash, ts
    FROM `+table+`
    WHERE tenant_id=$1
    ORDER BY ts DESC
    LIMIT $2`, tenant, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []receipts.ChainRecord{}
	for rows.Next() {
		var id uuid.UUID
		var body []byte
		var prev *string
		var hash string
		var ts time.Time
		if err := rows.Scan(&id, &body, &prev, &hash, &ts); err != nil {
			return nil, err
		}
		prevVal := ""
		if prev != nil {
			prevVal = *prev
		}
		out = append(out, receipts.ChainRecord{
			ID:       id.String(),
			Body:     body,
			PrevHash: prevVal,
			Hash:     hash,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func itoa(i int) string {
	// tiny helper to avoid importing strconv across the file for V0 minimalism
	// (but still deterministic).
	digits := "0123456789"
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := make([]byte, 0, 12)
	for i > 0 {
		buf = append(buf, digits[i%10])
		i /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}
