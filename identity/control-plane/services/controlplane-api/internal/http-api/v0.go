package httpapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

type Server struct {
	Logger *slog.Logger
	Store  *dbstore.Store
}

type policyResponse struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Version    int             `json:"version"`
	Active     bool            `json:"active"`
	Policy     json.RawMessage `json:"policy"`
	PolicyHash string          `json:"policy_hash"`
	UpdatedAt  time.Time       `json:"updated_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

func policyResponseFrom(p dbstore.Policy) policyResponse {
	return policyResponse{
		ID:         p.ID,
		Name:       p.Name,
		Version:    p.Version,
		Active:     p.Active,
		Policy:     p.Policy,
		PolicyHash: p.PolicyHash,
		UpdatedAt:  p.UpdatedAt,
		CreatedAt:  p.CreatedAt,
	}
}

type ListReceiptsResponse struct {
	Items      []json.RawMessage `json:"items"`
	NextBefore string            `json:"next_before,omitempty"`
}

func registerV0(mux *http.ServeMux, logger *slog.Logger) {
	// Wire DB (V0 simple): create store per request via global singleton in closure.
	// In production, build a proper server struct in main and inject dependencies.
	// For V0, we keep it concise but safe (timeouts, validation).
	dsn := getenv("DATABASE_URL", "")
	if dsn == "" {
		logger.Warn("DATABASE_URL missing; controlplane endpoints will be limited")
	}
	ctx := context.Background()
	db, err := stor.Connect(ctx, dsn)
	if err != nil {
		logger.Error("db connect failed", "err", err)
	}
	var store *dbstore.Store
	if db != nil {
		store = dbstore.New(db)
	}

	s := &Server{Logger: logger, Store: store}

	mux.HandleFunc("/v1/tools", s.handleTools)
	mux.HandleFunc("/v1/policies", s.handlePolicies)
	mux.HandleFunc("/v1/policies/active", s.handleActivePolicy)
	mux.HandleFunc("/v1/policies/activate", s.handleActivatePolicy)
	mux.HandleFunc("/v1/policies/", s.handlePolicyByID)
	mux.HandleFunc("/v1/policies/simulate", s.handleSimulatePolicy)
	mux.HandleFunc("/v1/receipts", s.handleReceipts)
	mux.HandleFunc("/v1/receipts/export", s.handleReceiptsExport)
	mux.HandleFunc("/v1/receipts/verify", s.handleReceiptsVerify)
}

func (s *Server) tenantFromRequest(r *http.Request) (uuid.UUID, error) {
	// V0 dev mode: tenant header.
	// Production: derive tenant from validated OIDC claims.
	tid := r.Header.Get("x-umbra-tenant-id")
	if tid == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(tid)
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		items, err := s.Store.ListTools(ctx, tenant, 50)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		writeJSON(w, map[string]interface{}{"items": items})
	case http.MethodPost:
		var body struct {
			Name   string          `json:"name"`
			Kind   string          `json:"kind"`
			Config json.RawMessage `json:"config"`
		}
		if err := decodeJSON(r, &body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if body.Name == "" || body.Kind == "" {
			http.Error(w, "name and kind required", 400)
			return
		}
		t, err := s.Store.CreateTool(ctx, tenant, body.Name, body.Kind, body.Config)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, t)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", http.StatusBadRequest)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		items, err := s.Store.ListPolicies(ctx, tenant, 50)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out := make([]policyResponse, 0, len(items))
		for _, p := range items {
			out = append(out, policyResponseFrom(p))
		}
		writeJSON(w, map[string]interface{}{"items": out})
	case http.MethodPost:
		var body struct {
			Name   string          `json:"name"`
			Policy json.RawMessage `json:"policy"`
		}
		if err := decodeJSON(r, &body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if body.Name == "" || len(body.Policy) == 0 {
			http.Error(w, "name and policy required", 400)
			return
		}

		// Validate the policy
		validationErrs, policyHash, policySize := policy.ValidatePolicyWithSize(body.Policy)
		if len(validationErrs) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"code":    policy.ErrorCodePolicyInvalid,
				"message": "policy validation failed",
				"errors":  validationErrs,
			}
			if reqID := r.Header.Get("x-request-id"); reqID != "" {
				response["request_id"] = reqID
			}
			writeJSON(w, response)
			s.Logger.Warn("policy validation failed",
				"tenant_id", tenant.String(),
				"policy_name", body.Name,
				"policy_size", policySize,
				"error_count", len(validationErrs),
			)
			return
		}

		// Create the policy
		p, err := s.Store.CreatePolicy(ctx, tenant, body.Name, body.Policy, policyHash)
		if err != nil {
			s.Logger.Error("policy creation failed", "err", err)
			http.Error(w, "db error", 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, policyResponseFrom(p))
		s.Logger.Info("policy created",
			"tenant_id", tenant.String(),
			"policy_id", p.ID.String(),
			"policy_name", p.Name,
			"policy_hash", p.PolicyHash,
		)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleActivatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", 503)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}
	var body struct {
		PolicyID string `json:"policy_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	pid, err := uuid.Parse(body.PolicyID)
	if err != nil {
		http.Error(w, "invalid policy_id", 400)
		return
	}
	s.activatePolicyByID(w, r, tenant, pid)
}

func (s *Server) handlePolicyByID(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", http.StatusBadRequest)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/policies/")
	path = strings.Trim(path, "/")
	if path == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	parts := strings.Split(path, "/")
	policyID, err := uuid.Parse(parts[0])
	if err != nil {
		http.Error(w, "invalid policy id", http.StatusBadRequest)
		return
	}

	if len(parts) == 2 && parts[1] == "activate" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.activatePolicyByID(w, r, tenant, policyID)
		return
	}

	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		p, err := s.Store.GetPolicy(ctx, tenant, policyID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "db error", 500)
			return
		}
		writeJSON(w, policyResponseFrom(p))
	case http.MethodPut:
		var body struct {
			Policy json.RawMessage `json:"policy"`
		}
		if err := decodeJSON(r, &body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if len(body.Policy) == 0 {
			http.Error(w, "policy required", 400)
			return
		}

		validationErrs, policyHash, policySize := policy.ValidatePolicyWithSize(body.Policy)
		if len(validationErrs) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"code":    policy.ErrorCodePolicyInvalid,
				"message": "policy validation failed",
				"errors":  validationErrs,
			}
			if reqID := r.Header.Get("x-request-id"); reqID != "" {
				response["request_id"] = reqID
			}
			writeJSON(w, response)
			s.Logger.Warn("policy validation failed",
				"tenant_id", tenant.String(),
				"policy_id", policyID.String(),
				"policy_size", policySize,
				"error_count", len(validationErrs),
			)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		existing, err := s.Store.GetPolicy(ctx, tenant, policyID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "db error", 500)
			return
		}
		if existing.Active {
			http.Error(w, "cannot update active policy", http.StatusConflict)
			return
		}

		p, err := s.Store.UpdatePolicy(ctx, tenant, policyID, body.Policy, policyHash)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		writeJSON(w, policyResponseFrom(p))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleActivePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", 503)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := s.Store.GetActivePolicy(ctx, tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "no active policy", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"version":     p.Version,
		"policy_hash": p.PolicyHash,
		"updated_at":  p.UpdatedAt,
	})
}

func (s *Server) activatePolicyByID(w http.ResponseWriter, r *http.Request, tenant uuid.UUID, policyID uuid.UUID) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	p, err := s.Store.GetPolicy(ctx, tenant, policyID)
	if err != nil {
		if errors.Is(err, stor.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", 500)
		return
	}

	validationErrs, _, _ := policy.ValidatePolicyWithSize(p.Policy)
	if len(validationErrs) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		response := map[string]interface{}{
			"code":    policy.ErrorCodePolicyInvalid,
			"message": "policy validation failed",
			"errors":  validationErrs,
		}
		if reqID := r.Header.Get("x-request-id"); reqID != "" {
			response["request_id"] = reqID
		}
		writeJSON(w, response)
		return
	}

	if err := s.Store.ActivatePolicy(ctx, tenant, policyID); err != nil {
		if errors.Is(err, stor.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

func (s *Server) handleSimulatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}

	var body struct {
		ActorID    string          `json:"actor_id,omitempty"`
		ActorType  string          `json:"actor_type,omitempty"`
		ActorRoles []string        `json:"actor_roles"`
		Method     string          `json:"method"`
		Path       string          `json:"path"`
		MCPServer  string          `json:"mcp_server,omitempty"`
		MCPTool    string          `json:"mcp_tool,omitempty"`
		MCPMethod  string          `json:"mcp_method,omitempty"`
		Policy     json.RawMessage `json:"policy,omitempty"`
	}
	if err := decodeJSON(r, &body); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Use provided policy or fetch active policy
	var policyData json.RawMessage
	var policyHash string
	var policyVersion int

	if len(body.Policy) > 0 {
		// Validate the supplied policy
		validationErrs, hash, _ := policy.ValidatePolicyWithSize(body.Policy)
		if len(validationErrs) > 0 {
			w.WriteHeader(http.StatusBadRequest)
			response := map[string]interface{}{
				"code":    policy.ErrorCodePolicyInvalid,
				"message": "policy validation failed",
				"errors":  validationErrs,
			}
			if reqID := r.Header.Get("x-request-id"); reqID != "" {
				response["request_id"] = reqID
			}
			writeJSON(w, response)
			return
		}
		policyData = body.Policy
		policyHash = hash
		policyVersion = 0 // Simulated policies don't have a version
	} else {
		if s.Store == nil {
			http.Error(w, "storage not configured", 503)
			return
		}
		// Fetch active policy
		policies, err := s.Store.ListPolicies(ctx, tenant, 50)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		found := false
		for _, p := range policies {
			if p.Active {
				policyData = p.Policy
				policyHash = p.PolicyHash
				policyVersion = p.Version
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "no active policy found", 404)
			return
		}
	}

	// Evaluate the policy
	var pol policy.Policy
	if err := json.Unmarshal(policyData, &pol); err != nil {
		http.Error(w, "policy parse error", 400)
		return
	}

	decision := policy.EvaluateABACV0(policy.RequestContext{
		ActorID:    body.ActorID,
		ActorType:  body.ActorType,
		ActorRoles: body.ActorRoles,
		Method:     body.Method,
		Path:       body.Path,
		MCPServer:  body.MCPServer,
		MCPTool:    body.MCPTool,
		MCPMethod:  body.MCPMethod,
	}, pol)

	response := map[string]interface{}{
		"decision":       decision.Decision,
		"reason":         decision.Reason,
		"policy_hash":    policyHash,
		"policy_version": policyVersion,
	}
	if decision.RuleIndex != nil {
		response["rule_index"] = *decision.RuleIndex
	}

	writeJSON(w, response)
}

func (s *Server) handleReceipts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", 503)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	q := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = &t
		}
	}

	items, next, err := s.Store.ListReceipts(ctx, tenant, limit, kind, q, before)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	resp := map[string]interface{}{"items": items}
	if next != nil {
		resp["next_before"] = next.UTC().Format(time.RFC3339)
	}
	writeJSON(w, resp)
}

func (s *Server) handleReceiptsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", 503)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}

	q := r.URL.Query()
	format := strings.ToLower(strings.TrimSpace(q.Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		http.Error(w, "invalid format", 400)
		return
	}

	var fromPtr *time.Time
	if from := strings.TrimSpace(q.Get("from")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			http.Error(w, "invalid from", 400)
			return
		}
		fromPtr = &t
	}
	var toPtr *time.Time
	if to := strings.TrimSpace(q.Get("to")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			http.Error(w, "invalid to", 400)
			return
		}
		toPtr = &t
	}

	decision := strings.ToLower(strings.TrimSpace(q.Get("decision")))
	if decision != "" && decision != "allow" && decision != "deny" {
		http.Error(w, "invalid decision", 400)
		return
	}

	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	filters := dbstore.ExportFilters{
		From:       fromPtr,
		To:         toPtr,
		ActorID:    q.Get("actor_id"),
		Tool:       q.Get("tool"),
		Decision:   decision,
		RequestID:  q.Get("request_id"),
		DecisionID: q.Get("decision_id"),
		Limit:      limit,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if format == "csv" {
		w.Header().Set("content-type", "text/csv")
		w.Header().Set("content-disposition", "attachment; filename=receipts_export.csv")
		if err := s.Store.ExportReceiptsCSV(ctx, tenant, filters, w); err != nil {
			http.Error(w, "db error", 500)
		}
		return
	}

	items, err := s.Store.ExportReceipts(ctx, tenant, filters)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	writeJSON(w, map[string]interface{}{
		"schema_version": "v1",
		"items":          items,
	})
}

func (s *Server) handleReceiptsVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Store == nil {
		http.Error(w, "storage not configured", 503)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		http.Error(w, "missing/invalid x-umbra-tenant-id", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "all"
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	verifyKind := func(k string) (receipts.VerifyResult, error) {
		records, err := s.Store.ListReceiptChain(ctx, tenant, limit, k)
		if err != nil {
			return receipts.VerifyResult{}, err
		}
		return receipts.VerifyChain(records), nil
	}

	switch kind {
	case "decision", "invocation":
		res, err := verifyKind(kind)
		if err != nil {
			if errors.Is(err, stor.ErrNotFound) {
				http.Error(w, "invalid kind", 400)
				return
			}
			http.Error(w, "db error", 500)
			return
		}
		writeJSON(w, map[string]interface{}{
			"ok":      res.OK,
			"checked": res.Checked,
			"kind":    kind,
			"failure": res.Failure,
		})
	case "all":
		res, err := verifyKind("decision")
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		if !res.OK {
			writeJSON(w, map[string]interface{}{
				"ok":      false,
				"checked": res.Checked,
				"kind":    "decision",
				"failure": res.Failure,
			})
			return
		}
		resInv, err := verifyKind("invocation")
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		if !resInv.OK {
			writeJSON(w, map[string]interface{}{
				"ok":      false,
				"checked": resInv.Checked,
				"kind":    "invocation",
				"failure": resInv.Failure,
			})
			return
		}
		writeJSON(w, map[string]interface{}{
			"ok":      true,
			"checked": res.Checked + resInv.Checked,
			"kind":    "all",
		})
	default:
		http.Error(w, "invalid kind", 400)
	}
}

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeReceiptsCSV(w http.ResponseWriter, items []dbstore.ExportRecord) {
	writer := csv.NewWriter(w)
	dbstore.WriteReceiptsCSVHeader(writer)
	for _, item := range items {
		dbstore.WriteReceiptsCSVRecord(writer, item)
	}
	writer.Flush()
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
