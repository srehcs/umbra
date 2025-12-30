package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/policy"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/pdp/internal/storage"
)

type receiptBody struct {
	Actor         protocol.Actor `json:"actor"`
	Tool          protocol.Tool  `json:"tool"`
	Decision      string         `json:"decision"`
	Reason        string         `json:"reason"`
	RuleIndex     *int           `json:"rule_index,omitempty"`
	PolicyHash    string         `json:"policy_hash,omitempty"`
	PolicyVersion int            `json:"policy_version,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	TraceID       string         `json:"trace_id,omitempty"`
	SpanID        string         `json:"span_id,omitempty"`
}

type errorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

func registerV0(mux *http.ServeMux, logger *slog.Logger) {
	dsn := os.Getenv("DATABASE_URL")
	db, err := stor.Connect(context.Background(), dsn)
	if err != nil {
		logger.Error("db connect failed", "err", err)
	}
	var store *dbstore.Store
	if db != nil {
		store = dbstore.New(db)
	}

	tracer := otel.Tracer("umbra.pdp")

	mux.HandleFunc("/v1/decision", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		var req protocol.DecisionRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "POLICY_INVALID", "invalid json", "", "")
			return
		}

		requestID, traceID, spanID := "", "", ""
		if req.Trace != nil {
			requestID = req.Trace.RequestID
			traceID = req.Trace.TraceID
			spanID = req.Trace.SpanID
		}
		reqLogger := logger.With("request_id", requestID)
		if traceID != "" {
			reqLogger = reqLogger.With("trace_id", traceID)
		}

		tenantID, err := uuid.Parse(req.Tenant.TenantID)
		if err != nil || tenantID == uuid.Nil {
			writeError(w, http.StatusBadRequest, "POLICY_INVALID", "invalid tenant_id", requestID, traceID)
			return
		}

		ctx, span := tracer.Start(ctx, "pdp.decision")
		span.SetAttributes(
			attribute.String("umbra.tenant_id", tenantID.String()),
			attribute.String("umbra.tool", req.Tool.Name),
			attribute.String("umbra.method", req.Tool.Method),
			attribute.String("umbra.endpoint", req.Tool.Endpoint),
			attribute.String("umbra.request_id", requestID),
		)
		defer span.End()

		// Default deny if store missing
		if store == nil {
			resp := protocol.DecisionResponse{
				Decision:   "deny",
				DecisionID: uuid.NewString(),
				Reason:     "storage unavailable (default deny)",
				RequestID:  requestID,
				TraceID:    traceID,
				SpanID:     spanID,
			}
			writeJSON(w, resp)
			return
		}

		// Load active policy
		ap, err := store.GetActivePolicy(ctx, tenantID)
		if err != nil {
			resp := protocol.DecisionResponse{
				Decision:   "deny",
				DecisionID: uuid.NewString(),
				Reason:     "no active policy (default deny)",
				RequestID:  requestID,
				TraceID:    traceID,
				SpanID:     spanID,
			}
			writeDecisionReceipt(ctx, reqLogger, store, tenantID, uuid.New(), requestID, "", "deny", receiptBody{
				Actor: req.Actor, Tool: req.Tool, Decision: "deny", Reason: "no active policy (default deny)",
				RequestID: requestID, TraceID: traceID, SpanID: spanID,
			}, traceID, spanID)
			writeJSON(w, resp)
			return
		}

		var pol policy.Policy
		if err := json.Unmarshal(ap.Policy, &pol); err != nil {
			writeError(w, http.StatusInternalServerError, "POLICY_INVALID", "invalid policy json", requestID, traceID)
			return
		}

		var mcpServer, mcpTool, mcpMethod string
		if req.MCP != nil {
			mcpServer = req.MCP.Server
			mcpTool = req.MCP.Tool
			mcpMethod = req.MCP.Method
		}
		d := policy.EvaluateABACV0(policy.RequestContext{
			ActorID:    req.Actor.ID,
			ActorType:  req.Actor.Type,
			ActorRoles: req.Actor.Roles,
			Method:     req.Tool.Method,
			Path:       req.Tool.Endpoint,
			MCPServer:  mcpServer,
			MCPTool:    mcpTool,
			MCPMethod:  mcpMethod,
		}, pol)

		decisionID := uuid.New()
		resp := protocol.DecisionResponse{
			Decision:      d.Decision,
			DecisionID:    decisionID.String(),
			PolicyVersion: pol.Version,
			PolicyHash:    ap.PolicyHash,
			RuleIndex:     d.RuleIndex,
			Reason:        d.Reason,
			Obligations:   []protocol.Obligation{},
			RequestID:     requestID,
			TraceID:       traceID,
			SpanID:        spanID,
		}

		writeDecisionReceipt(ctx, reqLogger, store, tenantID, decisionID, requestID, ap.PolicyHash, d.Decision, receiptBody{
			Actor:         req.Actor,
			Tool:          req.Tool,
			Decision:      d.Decision,
			Reason:        d.Reason,
			RuleIndex:     d.RuleIndex,
			PolicyHash:    ap.PolicyHash,
			PolicyVersion: pol.Version,
			RequestID:     requestID,
			TraceID:       traceID,
			SpanID:        spanID,
		}, traceID, spanID)

		reqLogger.Info("decision evaluated", "decision_id", decisionID.String(), "decision", d.Decision, "policy_hash", ap.PolicyHash)

		writeJSON(w, resp)
	})
}

func writeDecisionReceipt(ctx context.Context, logger *slog.Logger, store *dbstore.Store, tenant uuid.UUID, decisionID uuid.UUID, requestID string, policyHash string, decision string, body receiptBody, traceID, spanID string) {
	if store == nil {
		return
	}
	bodyBytes, err := receipts.CanonicalJSON(body)
	if err != nil {
		logger.Error("receipt canonical json failed", "err", err)
		return
	}
	prev, _ := store.LastDecisionHash(ctx, tenant)
	hash := receipts.HashBytes(append([]byte(prev), bodyBytes...))
	if err := store.InsertDecisionReceipt(ctx, tenant, decisionID, requestID, policyHash, decision, bodyBytes, prev, hash, traceID, spanID); err != nil {
		logger.Error("insert decision receipt failed", "err", err)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message, requestID, traceID string) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		ErrorCode: code,
		Message:   message,
		RequestID: requestID,
		TraceID:   traceID,
	})
}
