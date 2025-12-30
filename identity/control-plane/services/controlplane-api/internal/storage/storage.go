package storage

import (
	"context"
	"encoding/json"
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

	// where clause helpers
	beforeClause := ""
	argsD := []interface{}{tenant, limit}
	argsI := []interface{}{tenant, limit}
	if before != nil {
		beforeClause = " AND ts < $3 "
		argsD = []interface{}{tenant, limit, *before}
		argsI = []interface{}{tenant, limit, *before}
	}

	// Decision receipts query (filter q over a few fields; V0 pragmatic)
	dsql := `
    SELECT ts, jsonb_build_object(
      'kind','decision',
      'id', id,
      'decision_id', decision_id,
      'request_id', request_id,
      'policy_hash', policy_hash,
      'decision', decision,
      'hash', hash,
      'prev_hash', prev_hash,
      'trace_id', trace_id,
      'span_id', span_id
    ) AS obj
    FROM receipts_decision
    WHERE tenant_id=$1` + beforeClause + `
    ORDER BY ts DESC
    LIMIT $2`

	// Invocation receipts query
	isql := `
    SELECT ts, jsonb_build_object(
      'kind','invocation',
      'id', id,
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
    FROM receipts_invocation
    WHERE tenant_id=$1` + beforeClause + `
    ORDER BY ts DESC
    LIMIT $2`

	out := []map[string]interface{}{}
	// Decisions
	if kind == "" || kind == "decision" || kind == "all" {
		drows, err := s.db.Query(ctx, dsql, argsD...)
		if err != nil {
			return nil, nil, err
		}
		defer drows.Close()
		for drows.Next() {
			var ts time.Time
			var obj []byte
			if err := drows.Scan(&ts, &obj); err != nil {
				return nil, nil, err
			}
			var m map[string]interface{}
			_ = json.Unmarshal(obj, &m)
			m["ts"] = ts
			out = append(out, m)
		}
		if err := drows.Err(); err != nil {
			return nil, nil, err
		}
	}

	// Invocations
	if kind == "" || kind == "invocation" || kind == "all" {
		irows, err := s.db.Query(ctx, isql, argsI...)
		if err != nil {
			return nil, nil, err
		}
		defer irows.Close()
		for irows.Next() {
			var ts time.Time
			var obj []byte
			if err := irows.Scan(&ts, &obj); err != nil {
				return nil, nil, err
			}
			var m map[string]interface{}
			_ = json.Unmarshal(obj, &m)
			m["ts"] = ts
			out = append(out, m)
		}
		if err := irows.Err(); err != nil {
			return nil, nil, err
		}
	}

	// Sort merged by ts desc (V0)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			ti, _ := out[i]["ts"].(time.Time)
			tj, _ := out[j]["ts"].(time.Time)
			if tj.After(ti) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	// Apply q filtering post-merge (simple, safe). Server-side enough for V0.
	if q != "" {
		filtered := []map[string]interface{}{}
		for _, r := range out {
			b, _ := json.Marshal(r)
			if strings.Contains(strings.ToLower(string(b)), q) {
				filtered = append(filtered, r)
			}
		}
		out = filtered
	}

	if len(out) > limit {
		out = out[:limit]
	}

	// next_before: oldest ts in page
	var next *time.Time
	if len(out) > 0 {
		t, _ := out[len(out)-1]["ts"].(time.Time)
		next = &t
	}

	items := make([]json.RawMessage, 0, len(out))
	for _, r := range out {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, b)
	}
	return items, next, nil
}

func (s *Store) listReceiptChain(ctx context.Context, tenant uuid.UUID, limit int, table string) ([]receipts.ChainRecord, error) {
	rows, err := s.db.Query(ctx, `
    SELECT id, body_json, prev_hash, hash, ts
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
