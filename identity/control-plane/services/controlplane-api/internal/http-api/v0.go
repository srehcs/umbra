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
	"sync"
	"time"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"

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

type fieldError = protocol.ErrorDetail

type receiptIngestRequest struct {
	Kind          string          `json:"kind"`
	RequestID     string          `json:"request_id"`
	DecisionID    string          `json:"decision_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	SpanID        string          `json:"span_id,omitempty"`
	SignatureAlg  string          `json:"signature_alg,omitempty"`
	SignatureKid  string          `json:"signature_kid,omitempty"`
	Signature     string          `json:"signature,omitempty"`
	SignedAt      string          `json:"signed_at,omitempty"`
	Decision      string          `json:"decision,omitempty"`
	PolicyHash    string          `json:"policy_hash,omitempty"`
	PolicyVersion *int            `json:"policy_version,omitempty"`
	ToolName      string          `json:"tool_name,omitempty"`
	Method        string          `json:"method,omitempty"`
	Path          string          `json:"path,omitempty"`
	Outcome       string          `json:"outcome,omitempty"`
	StatusCode    *int            `json:"status_code,omitempty"`
	LatencyMs     *int            `json:"latency_ms,omitempty"`
	Body          json.RawMessage `json:"body"`
}

type receiptIngestResponse struct {
	ReceiptID string `json:"receipt_id"`
	Hash      string `json:"hash"`
	PrevHash  string `json:"prev_hash,omitempty"`
}

const requestIDConflictMessage = "request_id conflicts with existing receipt"

func registerV0(mux *http.ServeMux, logger *slog.Logger) error {
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
		signer, signerPolicy, signErr := receipts.NewSignerFromEnvWithPolicy()
		if signErr != nil {
			if signerPolicy.Required || receipts.IsReceiptSigningUnavailable(signErr) {
				return signErr
			}
			logger.Error("receipt signer init failed; continuing without signing", "err", signErr)
			store = dbstore.New(db)
		}
		if signer != nil {
			logger.Info("receipt signing enabled for controlplane-api")
			store = dbstore.NewWithSignerPolicy(db, signer, signerPolicy.Required)
		}
		if store == nil {
			store = dbstore.New(db)
		}
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
	return nil
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
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		items, err := s.Store.ListTools(ctx, tenant, 50)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeInvalidJSON(w, r)
			return
		}
		if body.Name == "" || body.Kind == "" {
			writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "name and kind required", nil, r)
			return
		}
		t, err := s.Store.CreateTool(ctx, tenant, body.Name, body.Kind, body.Config)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, t)
	default:
		writeMethodNotAllowed(w, r)
	}
}

func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		items, err := s.Store.ListPolicies(ctx, tenant, 50)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeInvalidJSON(w, r)
			return
		}
		if body.Name == "" || len(body.Policy) == 0 {
			writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "name and policy required", nil, r)
			return
		}

		// Validate the policy
		validationErrs, policyHash, policySize := policy.ValidatePolicyWithSize(body.Policy)
		if len(validationErrs) > 0 {
			writeErrorResponse(w, http.StatusBadRequest, policy.ErrorCodePolicyInvalid, "policy validation failed", policyErrorsToFieldErrors(validationErrs), r)
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
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
		writeMethodNotAllowed(w, r)
	}
}

func (s *Server) handleActivatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, r)
		return
	}
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}
	var body struct {
		PolicyID string `json:"policy_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeInvalidJSON(w, r)
		return
	}
	pid, err := uuid.Parse(body.PolicyID)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid policy_id", nil, r)
		return
	}
	s.activatePolicyByID(w, r, tenant, pid)
}

func (s *Server) handlePolicyByID(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
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
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid policy id", nil, r)
		return
	}

	if len(parts) == 2 && parts[1] == "activate" {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, r)
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
				writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "not found", nil, r)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		writeJSON(w, policyResponseFrom(p))
	case http.MethodPut:
		var body struct {
			Policy json.RawMessage `json:"policy"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeInvalidJSON(w, r)
			return
		}
		if len(body.Policy) == 0 {
			writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "policy required", nil, r)
			return
		}

		validationErrs, policyHash, policySize := policy.ValidatePolicyWithSize(body.Policy)
		if len(validationErrs) > 0 {
			writeErrorResponse(w, http.StatusBadRequest, policy.ErrorCodePolicyInvalid, "policy validation failed", policyErrorsToFieldErrors(validationErrs), r)
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
				writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "not found", nil, r)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		if existing.Active {
			writeErrorResponse(w, http.StatusConflict, protocol.ErrorCodeConflict, "cannot update active policy", nil, r)
			return
		}

		p, err := s.Store.UpdatePolicy(ctx, tenant, policyID, body.Policy, policyHash)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		writeJSON(w, policyResponseFrom(p))
	default:
		writeMethodNotAllowed(w, r)
	}
}

func (s *Server) handleActivePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, r)
		return
	}
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := s.Store.GetActivePolicy(ctx, tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "no active policy", nil, r)
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "not found", nil, r)
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
		return
	}

	validationErrs, _, _ := policy.ValidatePolicyWithSize(p.Policy)
	if len(validationErrs) > 0 {
		writeErrorResponse(w, http.StatusBadRequest, policy.ErrorCodePolicyInvalid, "policy validation failed", policyErrorsToFieldErrors(validationErrs), r)
		return
	}

	if err := s.Store.ActivatePolicy(ctx, tenant, policyID); err != nil {
		if errors.Is(err, stor.ErrNotFound) {
			writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "not found", nil, r)
			return
		}
		writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

func (s *Server) handleSimulatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
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
		writeInvalidJSON(w, r)
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
			errs := make([]fieldError, 0, len(validationErrs))
			for _, err := range validationErrs {
				errs = append(errs, fieldError{Field: err.Path, Message: err.Message})
			}
			writeErrorResponse(w, http.StatusBadRequest, policy.ErrorCodePolicyInvalid, "policy validation failed", errs, r)
			return
		}
		policyData = body.Policy
		policyHash = hash
		policyVersion = 0 // Simulated policies don't have a version
	} else {
		if s.Store == nil {
			writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
			return
		}
		// Fetch active policy
		policies, err := s.Store.ListPolicies(ctx, tenant, 50)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeErrorResponse(w, http.StatusNotFound, protocol.ErrorCodeNotFound, "no active policy found", nil, r)
			return
		}
	}

	// Evaluate the policy
	var pol policy.Policy
	if err := json.Unmarshal(policyData, &pol); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "policy parse error", nil, r)
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
	switch r.Method {
	case http.MethodGet:
		s.handleReceiptsList(w, r)
	case http.MethodPost:
		s.handleReceiptsIngest(w, r)
	default:
		writeMethodNotAllowed(w, r)
	}
}

func (s *Server) handleReceiptsList(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
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
		writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
		return
	}

	resp := map[string]interface{}{"items": items}
	if next != nil {
		resp["next_before"] = next.UTC().Format(time.RFC3339)
	}
	writeJSON(w, resp)
}

func (s *Server) handleReceiptsIngest(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}

	var body receiptIngestRequest
	if err := decodeJSON(r, &body); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "invalid json", nil, r)
		return
	}
	if errs := validateReceiptIngest(body); len(errs) > 0 {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "receipt validation failed", errs, r)
		return
	}
	if len(body.Body) > maxReceiptBodyBytes {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_TOO_LARGE", "receipt body too large", nil, r)
		return
	}
	if containsArgsField(body.Body) {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "receipt body must not include raw tool args", nil, r)
		return
	}

	canonicalBytes, err := receipts.CanonicalJSONBytes(body.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", receiptCanonicalizationError(err), nil, r)
		return
	}
	idempotencyBytes, err := canonicalizeIdempotencyPayload(body, canonicalBytes)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "receipt body must be valid JSON", nil, r)
		return
	}
	canonicalBody := json.RawMessage(canonicalBytes)
	idempotencyBody := json.RawMessage(idempotencyBytes)
	dedupeSince := receipts.RequestIDDedupeSince(time.Now().UTC(), requestIDDedupeWindow(s.Logger))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch body.Kind {
	case "decision":
		decisionID, err := uuid.Parse(body.DecisionID)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "invalid decision_id", nil, r)
			return
		}
		result, err := s.Store.InsertDecisionReceiptIdempotent(ctx, tenant, body.RequestID, decisionID, body.PolicyHash, body.Decision, canonicalBody, idempotencyBody, body.TraceID, body.SpanID, dedupeSince, receiptChainLockScope(s.Logger))
		if err != nil {
			if receipts.IsReceiptSigningUnavailable(err) {
				writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", nil, r)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		if result.Outcome == receipts.IdempotencyReplayed {
			w.WriteHeader(http.StatusOK)
			writeJSON(w, receiptIngestResponse{ReceiptID: result.Record.ID.String(), Hash: result.Record.Hash, PrevHash: result.Record.PrevHash})
			return
		}
		if result.Outcome == receipts.IdempotencyConflict {
			writeErrorResponse(w, http.StatusConflict, protocol.ErrorCodeConflict, requestIDConflictMessage, requestIDConflictDetails(), r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, receiptIngestResponse{ReceiptID: result.Record.ID.String(), Hash: result.Record.Hash, PrevHash: result.Record.PrevHash})
	case "invocation":
		var decisionID *uuid.UUID
		if body.DecisionID != "" {
			parsed, err := uuid.Parse(body.DecisionID)
			if err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "invalid decision_id", nil, r)
				return
			}
			decisionID = &parsed
		}
		latencyMs := 0
		if body.LatencyMs != nil {
			latencyMs = *body.LatencyMs
		}
		result, err := s.Store.InsertInvocationReceiptIdempotent(ctx, tenant, body.RequestID, decisionID, body.ToolName, body.Method, body.Path, body.Outcome, body.StatusCode, latencyMs, canonicalBody, idempotencyBody, body.TraceID, body.SpanID, dedupeSince, receiptChainLockScope(s.Logger))
		if err != nil {
			if receipts.IsReceiptSigningUnavailable(err) {
				writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeReceiptSigningUnavailable, "receipt signing unavailable", nil, r)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
			return
		}
		if result.Outcome == receipts.IdempotencyReplayed {
			w.WriteHeader(http.StatusOK)
			writeJSON(w, receiptIngestResponse{ReceiptID: result.Record.ID.String(), Hash: result.Record.Hash, PrevHash: result.Record.PrevHash})
			return
		}
		if result.Outcome == receipts.IdempotencyConflict {
			writeErrorResponse(w, http.StatusConflict, protocol.ErrorCodeConflict, requestIDConflictMessage, requestIDConflictDetails(), r)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, receiptIngestResponse{ReceiptID: result.Record.ID.String(), Hash: result.Record.Hash, PrevHash: result.Record.PrevHash})
	default:
		writeErrorResponse(w, http.StatusBadRequest, "RECEIPT_INVALID", "invalid kind", nil, r)
	}
}

const maxReceiptBodyBytes = 64 * 1024

func validateReceiptIngest(body receiptIngestRequest) []fieldError {
	var errs []fieldError
	if strings.TrimSpace(body.Kind) == "" {
		errs = append(errs, fieldError{Field: "kind", Message: "required"})
	}
	if strings.TrimSpace(body.RequestID) == "" {
		errs = append(errs, fieldError{Field: "request_id", Message: "required"})
	}
	if len(body.Body) == 0 {
		errs = append(errs, fieldError{Field: "body", Message: "required"})
	}
	if strings.TrimSpace(body.SignatureAlg) != "" {
		errs = append(errs, fieldError{Field: "signature_alg", Message: "must be omitted; server-managed"})
	}
	if strings.TrimSpace(body.SignatureKid) != "" {
		errs = append(errs, fieldError{Field: "signature_kid", Message: "must be omitted; server-managed"})
	}
	if strings.TrimSpace(body.Signature) != "" {
		errs = append(errs, fieldError{Field: "signature", Message: "must be omitted; server-managed"})
	}
	if strings.TrimSpace(body.SignedAt) != "" {
		errs = append(errs, fieldError{Field: "signed_at", Message: "must be omitted; server-managed"})
	}

	switch body.Kind {
	case "decision":
		if strings.TrimSpace(body.DecisionID) == "" {
			errs = append(errs, fieldError{Field: "decision_id", Message: "required"})
		}
		if body.Decision != "allow" && body.Decision != "deny" {
			errs = append(errs, fieldError{Field: "decision", Message: "must be allow or deny"})
		}
		if strings.TrimSpace(body.PolicyHash) == "" {
			errs = append(errs, fieldError{Field: "policy_hash", Message: "required"})
		}
	case "invocation":
		if strings.TrimSpace(body.ToolName) == "" {
			errs = append(errs, fieldError{Field: "tool_name", Message: "required"})
		}
		if strings.TrimSpace(body.Method) == "" {
			errs = append(errs, fieldError{Field: "method", Message: "required"})
		}
		if strings.TrimSpace(body.Path) == "" {
			errs = append(errs, fieldError{Field: "path", Message: "required"})
		}
		if body.Outcome != "success" && body.Outcome != "error" && body.Outcome != "denied" {
			errs = append(errs, fieldError{Field: "outcome", Message: "must be success, error, or denied"})
		}
		if body.LatencyMs == nil || *body.LatencyMs < 0 {
			errs = append(errs, fieldError{Field: "latency_ms", Message: "required"})
		}
	default:
		if strings.TrimSpace(body.Kind) != "" {
			errs = append(errs, fieldError{Field: "kind", Message: "must be decision or invocation"})
		}
	}
	return errs
}

func containsArgsField(raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	_, hasArgs := m["args"]
	_, hasArguments := m["arguments"]
	return hasArgs || hasArguments
}

func writeErrorResponse(w http.ResponseWriter, status int, code string, message string, errs []fieldError, r *http.Request) {
	reqID := ensureRequestID(r)
	traceID := ""
	if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
		traceID = sc.TraceID().String()
	}
	protocol.WriteErrorResponse(w, status, code, message, reqID, "", traceID, errs)
}

func requestIDConflictDetails() []fieldError {
	return []fieldError{{Field: "request_id", Message: "already used with different payload"}}
}

func receiptCanonicalizationError(err error) string {
	switch {
	case errors.Is(err, receipts.ErrCanonicalJSONNonASCII):
		return "receipt body must be ASCII-only JSON"
	case errors.Is(err, receipts.ErrCanonicalJSONFloat):
		return "receipt body must not include floating point numbers"
	case errors.Is(err, receipts.ErrCanonicalJSONTrailing):
		return "receipt body must be a single JSON value"
	default:
		return "receipt body must be valid JSON"
	}
}

func canonicalizeIdempotencyPayload(body receiptIngestRequest, canonicalBody []byte) ([]byte, error) {
	payload := receipts.IdempotencyPayload{
		Kind:       body.Kind,
		RequestID:  body.RequestID,
		DecisionID: body.DecisionID,
		Decision:   body.Decision,
		PolicyHash: body.PolicyHash,
		ToolName:   body.ToolName,
		Method:     body.Method,
		Path:       body.Path,
		Outcome:    body.Outcome,
		Body:       json.RawMessage(canonicalBody),
	}
	return receipts.CanonicalizeIdempotencyPayload(payload)
}

func writeMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeErrorResponse(w, http.StatusMethodNotAllowed, protocol.ErrorCodeMethodNotAllowed, "method not allowed", nil, r)
}

func writeInvalidTenant(w http.ResponseWriter, r *http.Request) {
	writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidTenant, "missing/invalid x-umbra-tenant-id", nil, r)
}

func writeInvalidJSON(w http.ResponseWriter, r *http.Request) {
	writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidJSON, "invalid json", nil, r)
}

func policyErrorsToFieldErrors(errs []policy.ValidationError) []fieldError {
	if len(errs) == 0 {
		return nil
	}
	out := make([]fieldError, 0, len(errs))
	for _, err := range errs {
		out = append(out, fieldError{Field: err.Path, Message: err.Message})
	}
	return out
}

func ensureRequestID(r *http.Request) string {
	reqID := strings.TrimSpace(r.Header.Get("x-umbra-request-id"))
	if reqID == "" {
		reqID = strings.TrimSpace(r.Header.Get("x-request-id"))
	}
	if reqID == "" {
		reqID = uuid.NewString()
	}
	r.Header.Set("x-umbra-request-id", reqID)
	r.Header.Set("x-request-id", reqID)
	return reqID
}

var dedupeWindowOnce sync.Once
var dedupeWindow time.Duration
var chainScopeOnce sync.Once
var chainScope string

func requestIDDedupeWindow(logger *slog.Logger) time.Duration {
	dedupeWindowOnce.Do(func() {
		window, err := receipts.ResolveRequestIDDedupeWindow(os.Getenv("UMBRA_REQUEST_ID_DEDUPE_WINDOW"))
		if err != nil {
			logger.Warn("invalid UMBRA_REQUEST_ID_DEDUPE_WINDOW; using default", "err", err)
		}
		dedupeWindow = window
	})
	if dedupeWindow == 0 {
		return receipts.DefaultRequestIDDedupeWindow
	}
	return dedupeWindow
}

func receiptChainLockScope(logger *slog.Logger) string {
	chainScopeOnce.Do(func() {
		scope, err := receipts.ResolveChainLockScope(os.Getenv("UMBRA_RECEIPT_CHAIN_LOCK_SCOPE"))
		if err != nil {
			logger.Warn("invalid UMBRA_RECEIPT_CHAIN_LOCK_SCOPE; using default", "err", err)
		}
		chainScope = scope
	})
	if chainScope == "" {
		return "tenant"
	}
	return chainScope
}

func (s *Server) handleReceiptsExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, r)
		return
	}
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
		return
	}

	q := r.URL.Query()
	format := strings.ToLower(strings.TrimSpace(q.Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid format", nil, r)
		return
	}

	var fromPtr *time.Time
	if from := strings.TrimSpace(q.Get("from")); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid from", nil, r)
			return
		}
		fromPtr = &t
	}
	var toPtr *time.Time
	if to := strings.TrimSpace(q.Get("to")); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid to", nil, r)
			return
		}
		toPtr = &t
	}

	decision := strings.ToLower(strings.TrimSpace(q.Get("decision")))
	if decision != "" && decision != "allow" && decision != "deny" {
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid decision", nil, r)
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
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
		}
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := s.Store.ExportReceiptsJSON(ctx, tenant, filters, w); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
	}
}

func (s *Server) handleReceiptsVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, r)
		return
	}
	if s.Store == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, protocol.ErrorCodeStorageUnavailable, "storage not configured", nil, r)
		return
	}
	tenant, err := s.tenantFromRequest(r)
	if err != nil || tenant == uuid.Nil {
		writeInvalidTenant(w, r)
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
				writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid kind", nil, r)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
			writeErrorResponse(w, http.StatusInternalServerError, protocol.ErrorCodeDBError, "db error", nil, r)
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
		writeErrorResponse(w, http.StatusBadRequest, protocol.ErrorCodeInvalidRequest, "invalid kind", nil, r)
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
