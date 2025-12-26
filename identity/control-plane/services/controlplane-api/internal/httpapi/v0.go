package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"

	"github.com/google/uuid"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/controlplane-api/internal/storage"
)

type Server struct {
	Logger *slog.Logger
	Store  *dbstore.Store
}

type ListReceiptsResponse struct {
  Items []json.RawMessage `json:"items"`
  NextBefore string `json:"next_before,omitempty"`
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
	mux.HandleFunc("/v1/policies/activate", s.handleActivatePolicy)
	mux.HandleFunc("/v1/policies/simulate", s.handleSimulatePolicy)
	mux.HandleFunc("/v1/receipts", s.handleReceipts)
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
		items, err := s.Store.ListPolicies(ctx, tenant, 50)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		writeJSON(w, map[string]interface{}{"items": items})
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
		h := sha256.Sum256(body.Policy)
		ph := hex.EncodeToString(h[:])
		p, err := s.Store.CreatePolicy(ctx, tenant, body.Name, body.Policy, ph)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, p)
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.Store.ActivatePolicy(ctx, tenant, pid); err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true})
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

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
