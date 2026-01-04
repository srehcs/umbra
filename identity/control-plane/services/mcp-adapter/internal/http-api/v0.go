package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/umbra-labs/agent-identity-control-plane/packages/go/protocol"
	"github.com/umbra-labs/agent-identity-control-plane/packages/go/receipts"
	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/mcp-adapter/internal/storage"
)

const (
	maxActorIDLen       = 128
	maxActorTypeLen     = 32
	maxActorSourceLen   = 32
	maxRoleLen          = 64
	maxRoleCount        = 32
	maxServerLen        = 200
	maxToolLen          = 200
	maxMethodLen        = 64
	maxWorkspaceLen     = 200
	maxSchemaVersionLen = 64
)

type PDPClient struct {
	BaseURL string
	Client  *http.Client
}

const maxPDPResponseBytes = 1 << 20

type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return e.Body }

func (c *PDPClient) Decide(ctx context.Context, payload protocol.DecisionRequest) (protocol.DecisionResponse, int, error) {
	var out protocol.DecisionResponse

	b, err := json.Marshal(payload)
	if err != nil {
		return out, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/decision", bytes.NewReader(b))
	if err != nil {
		return out, 0, err
	}
	req.Header.Set("content-type", "application/json")

	res, err := c.Client.Do(req)
	if err != nil {
		return out, 0, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, maxPDPResponseBytes))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return out, res.StatusCode, &httpError{Status: res.StatusCode, Body: string(body)}
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, res.StatusCode, err
	}
	return out, res.StatusCode, nil
}

type invocationStore interface {
	LastInvocationHash(ctx context.Context, tenant uuid.UUID) (string, error)
	InsertInvocationReceipt(ctx context.Context, tenant uuid.UUID, decisionID *uuid.UUID, requestID string, toolName string, method string, path string, outcome string, statusCode *int, latencyMs int, body json.RawMessage, prevHash string, hash string, traceID string, spanID string) error
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcRequestOut struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  interface{}     `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type rpcErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rpcErrorData struct {
	Error      rpcErrorBody `json:"error"`
	RequestID  string       `json:"request_id,omitempty"`
	DecisionID string       `json:"decision_id,omitempty"`
}

type toolCallParams struct {
	Name              string                 `json:"name"`
	Arguments         map[string]interface{} `json:"arguments,omitempty"`
	Server            string                 `json:"server,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	Workspace         string                 `json:"workspace,omitempty"`
	ArgsSchemaVersion string                 `json:"args_schema_version,omitempty"`
	Actor             *actorParams           `json:"actor,omitempty"`
}

type invocationMeta struct {
	ArgSizeBytes     int    `json:"arg_size_bytes,omitempty"`
	ArgSchemaVersion string `json:"arg_schema_version,omitempty"`
	ContentHash      string `json:"content_hash,omitempty"`
}

type invocationReceiptBody struct {
	ActorID        string          `json:"actor.id,omitempty"`
	ActorType      string          `json:"actor.type,omitempty"`
	ActorRoles     []string        `json:"actor.roles"`
	ActorSource    string          `json:"actor.source,omitempty"`
	ActorRoleCount int             `json:"actor.role_count,omitempty"`
	MCPServer      string          `json:"mcp.server,omitempty"`
	MCPTool        string          `json:"mcp.tool,omitempty"`
	MCPMethod      string          `json:"mcp.method,omitempty"`
	Workspace      string          `json:"workspace,omitempty"`
	PolicyHash     string          `json:"policy_hash,omitempty"`
	PolicyVersion  int             `json:"policy_version,omitempty"`
	Outcome        string          `json:"outcome"`
	StatusCode     *int            `json:"status_code,omitempty"`
	PDPLatencyMs   int             `json:"pdp_latency_ms"`
	ToolLatencyMs  int             `json:"tool_latency_ms"`
	TotalLatencyMs int             `json:"total_latency_ms"`
	Meta           *invocationMeta `json:"meta,omitempty"`
	StartedAt      string          `json:"started_at"`
	RequestID      string          `json:"request_id,omitempty"`
	PEPMode        string          `json:"pep.mode,omitempty"`
	Enforcement    string          `json:"enforcement.outcome,omitempty"`
	PDPStatus      string          `json:"pdp.status,omitempty"`
}

type mcpHandler struct {
	tracer        trace.Tracer
	logger        *slog.Logger
	store         invocationStore
	pdp           *PDPClient
	toolClient    *http.Client
	pepMode       string
	upstreamURL   string
	defaultTenant uuid.UUID
	defaultActor  actorIdentity
	serverName    string
}

type actorParams struct {
	ID     string   `json:"id,omitempty"`
	Type   string   `json:"type,omitempty"`
	Roles  []string `json:"roles,omitempty"`
	Source string   `json:"source,omitempty"`
}

type actorIdentity struct {
	ID     string
	Type   string
	Roles  []string
	Source string
}

func registerV0(mux *http.ServeMux, logger *slog.Logger) {
	tracer := otel.Tracer("umbra.pep.mcp")

	pepMode := strings.ToLower(getenv("PEP_MODE", "observe"))
	if pepMode != "enforce" && pepMode != "observe" {
		logger.Warn("invalid PEP_MODE, defaulting to observe", "value", pepMode)
		pepMode = "observe"
	}

	pdpTimeout := durationFromEnv("PDP_TIMEOUT_MS", 3000*time.Millisecond)
	toolTimeout := durationFromEnv("TOOL_TIMEOUT_MS", 5000*time.Millisecond)

	pdp := &PDPClient{
		BaseURL: getenv("PDP_URL", "http://pdp:8081"),
		Client:  &http.Client{Timeout: pdpTimeout},
	}

	upstreamURL := strings.TrimSpace(getenv("MCP_UPSTREAM_URL", ""))
	if upstreamURL == "" {
		logger.Warn("MCP_UPSTREAM_URL not set; forwarding will fail")
	}

	defaultTenant := getenv("MCP_TENANT_ID", "00000000-0000-0000-0000-000000000001")
	tenantID, err := uuid.Parse(defaultTenant)
	if err != nil || tenantID == uuid.Nil {
		logger.Warn("invalid MCP_TENANT_ID, defaulting to nil", "value", defaultTenant)
		tenantID = uuid.Nil
	}

	actorID := strings.TrimSpace(getenv("MCP_ACTOR_ID", "user-1"))
	actorType := strings.TrimSpace(getenv("MCP_ACTOR_TYPE", "agent"))
	roles := parseCSV(getenv("MCP_ACTOR_ROLES", ""))
	serverName := strings.TrimSpace(getenv("MCP_SERVER_NAME", "mcp.default"))

	db, err := stor.Connect(context.Background(), getenv("DATABASE_URL", "postgres://umbra:umbra@postgres:5432/umbra?sslmode=disable"))
	if err != nil {
		logger.Error("db connect failed", "err", err)
	}
	var store invocationStore
	if db != nil {
		store = dbstore.New(db)
	}

	h := &mcpHandler{
		tracer:        tracer,
		logger:        logger,
		store:         store,
		pdp:           pdp,
		toolClient:    &http.Client{Timeout: toolTimeout},
		pepMode:       pepMode,
		upstreamURL:   upstreamURL,
		defaultTenant: tenantID,
		defaultActor: actorIdentity{
			ID:     actorID,
			Type:   actorType,
			Roles:  roles,
			Source: "dev",
		},
		serverName: serverName,
	}

	mux.HandleFunc("/mcp", h.handleMCP)
}

func (h *mcpHandler) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRPCMethodNotAllowed(w)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeRPCBadRequest(w, nil, "invalid request", "")
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCBadRequest(w, nil, "invalid json", "")
		return
	}

	if strings.TrimSpace(req.Method) != "tools/call" {
		writeRPCMethodNotSupported(w, req.ID)
		return
	}

	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "invalid params", "BAD_PARAMS", "")
		return
	}

	toolName, err := normalizeIdentifier(params.Name, maxToolLen, false)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "tool name too long", "CONTEXT_TOO_LARGE", "")
		return
	}
	if toolName == "" {
		writeRPCError(w, http.StatusBadRequest, req.ID, "missing tool name", "BAD_PARAMS", "")
		return
	}

	requestID := strings.TrimSpace(params.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("x-umbra-request-id"))
	}
	if requestID == "" {
		requestID = uuid.NewString()
		params.RequestID = requestID
	}

	serverName, err := normalizeServerIdentifier(params.Server, maxServerLen)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "server id too long", "CONTEXT_TOO_LARGE", requestID)
		return
	}
	if serverName == "" {
		serverName, err = normalizeServerIdentifier(h.serverName, maxServerLen)
		if err != nil {
			writeRPCError(w, http.StatusBadRequest, req.ID, "server id too long", "CONTEXT_TOO_LARGE", requestID)
			return
		}
	}

	workspace, err := normalizeIdentifier(params.Workspace, maxWorkspaceLen, false)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "workspace too long", "CONTEXT_TOO_LARGE", requestID)
		return
	}
	if workspace == "" {
		if headerWorkspace := strings.TrimSpace(r.Header.Get("x-umbra-workspace")); headerWorkspace != "" {
			workspace, err = normalizeIdentifier(headerWorkspace, maxWorkspaceLen, false)
			if err != nil {
				writeRPCError(w, http.StatusBadRequest, req.ID, "workspace too long", "CONTEXT_TOO_LARGE", requestID)
				return
			}
		}
	}

	argSchemaVersion := strings.TrimSpace(params.ArgsSchemaVersion)
	if argSchemaVersion != "" && len(argSchemaVersion) > maxSchemaVersionLen {
		writeRPCError(w, http.StatusBadRequest, req.ID, "args schema version too long", "CONTEXT_TOO_LARGE", requestID)
		return
	}

	meta := buildInvocationMeta(params.Arguments, argSchemaVersion)

	actor, err := resolveActor(r, params, h.defaultActor)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "actor identity too long", "CONTEXT_TOO_LARGE", requestID)
		return
	}

	tenantID := h.defaultTenant
	if headerTenant := strings.TrimSpace(r.Header.Get("x-umbra-tenant-id")); headerTenant != "" {
		if parsed, err := uuid.Parse(headerTenant); err == nil {
			tenantID = parsed
		}
	}
	if tenantID == uuid.Nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "missing tenant", "MISSING_TENANT", requestID)
		return
	}

	ctx, span := h.tracer.Start(r.Context(), "pep.mcp")
	defer span.End()

	sc := trace.SpanContextFromContext(ctx)
	traceID := ""
	spanID := ""
	if sc.IsValid() {
		traceID = sc.TraceID().String()
		spanID = sc.SpanID().String()
	}

	logger := h.logger.With(
		"request_id", requestID,
		"actor_id", actor.ID,
		"actor_type", actor.Type,
		"actor_source", actor.Source,
	)
	if traceID != "" {
		logger = logger.With("trace_id", traceID, "span_id", spanID)
	}

	mcpMethod, err := normalizeIdentifier(req.Method, maxMethodLen, true)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "method too long", "CONTEXT_TOO_LARGE", requestID)
		return
	}

	span.SetAttributes(
		attribute.String("umbra.tenant_id", tenantID.String()),
		attribute.String("umbra.request_id", requestID),
		attribute.String("mcp.method", mcpMethod),
		attribute.String("mcp.tool", toolName),
		attribute.String("mcp.server", serverName),
		attribute.String("umbra.actor_id", actor.ID),
		attribute.String("umbra.actor_type", actor.Type),
	)

	started := time.Now()
	pdpStarted := time.Now()

	traceCtx := &protocol.TraceContext{RequestID: requestID}
	if traceID != "" {
		traceCtx.TraceID = traceID
		traceCtx.SpanID = spanID
	}

	decisionReq := protocol.DecisionRequest{
		Tenant: protocol.TenantContext{TenantID: tenantID.String()},
		Actor: protocol.Actor{
			Type:   actor.Type,
			ID:     actor.ID,
			Roles:  actor.Roles,
			Source: actor.Source,
		},
		Tool: protocol.Tool{
			Name:     toolName,
			Method:   mcpMethod,
			Endpoint: toolEndpoint(serverName, toolName),
		},
		MCP: &protocol.MCPContext{
			Server:    serverName,
			Tool:      toolName,
			Method:    mcpMethod,
			Workspace: workspace,
		},
		Trace: traceCtx,
	}

	mcpCtx := protocol.MCPContext{
		Server:    serverName,
		Tool:      toolName,
		Method:    mcpMethod,
		Workspace: workspace,
	}

	decision, status, err := h.pdp.Decide(ctx, decisionReq)
	pdpLatency := int(time.Since(pdpStarted).Milliseconds())
	pdpStatus := "ok"
	policyHash := ""
	policyVersion := 0

	var decisionID *uuid.UUID
	if err == nil {
		if parsed, err := uuid.Parse(decision.DecisionID); err == nil {
			decisionID = &parsed
		}
		policyHash = decision.PolicyHash
		policyVersion = decision.PolicyVersion
		logger = logger.With("decision_id", decision.DecisionID)
		span.SetAttributes(attribute.String("umbra.decision_id", decision.DecisionID))
		logger.Info("pdp decision received", "decision", decision.Decision)
	} else {
		pdpStatus = "unavailable"
		logger.Error("pdp decide failed", "err", err, "status", status)
	}

	forward := false
	enforcement := "blocked"
	outcome := "denied"
	statusCode := http.StatusForbidden

	if err != nil {
		if h.pepMode == "observe" {
			forward = true
			enforcement = "forwarded"
			outcome = "success"
			statusCode = http.StatusOK
		} else {
			statusCode = http.StatusServiceUnavailable
		}
	} else if strings.ToLower(decision.Decision) == "allow" {
		forward = true
		enforcement = "forwarded"
		outcome = "success"
		statusCode = http.StatusOK
	} else if h.pepMode == "observe" {
		forward = true
		enforcement = "forwarded"
		outcome = "denied"
		statusCode = http.StatusOK
	}

	toolLatency := 0
	var toolStatus *int

	if forward {
		forwardBody, err := buildForwardBody(req, params)
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, actor, mcpCtx, policyHash, policyVersion, outcome, intPtr(http.StatusBadRequest), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadRequest, req.ID, "invalid request", protocol.ErrorCodeBadRequest, requestID)
			return
		}

		toolStarted := time.Now()
		toolReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.upstreamURL, bytes.NewReader(forwardBody))
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, actor, mcpCtx, policyHash, policyVersion, "error", intPtr(http.StatusBadGateway), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadGateway, req.ID, "upstream unavailable", "UPSTREAM_UNAVAILABLE", requestID)
			return
		}
		toolReq.Header.Set("content-type", "application/json")

		toolRes, err := h.toolClient.Do(toolReq)
		toolLatency = int(time.Since(toolStarted).Milliseconds())
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, actor, mcpCtx, policyHash, policyVersion, "error", intPtr(http.StatusBadGateway), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadGateway, req.ID, "upstream error", protocol.ErrorCodeUpstreamError, requestID)
			return
		}
		defer toolRes.Body.Close()
		toolStatusVal := toolRes.StatusCode
		toolStatus = &toolStatusVal

		if outcome == "success" {
			if toolRes.StatusCode >= 400 {
				outcome = "error"
			}
		}
		writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, actor, mcpCtx, policyHash, policyVersion, outcome, toolStatus, pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
		if ct := toolRes.Header.Get("content-type"); ct != "" {
			w.Header().Set("content-type", ct)
		} else {
			w.Header().Set("content-type", "application/json")
		}
		w.WriteHeader(toolRes.StatusCode)
		_, _ = io.Copy(w, toolRes.Body)
		return
	}

	writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, actor, mcpCtx, policyHash, policyVersion, outcome, intPtr(statusCode), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)

	if err != nil {
		writeRPCError(w, http.StatusServiceUnavailable, req.ID, "policy unavailable", protocol.ErrorCodePolicyUnavailable, requestID)
		return
	}

	writeRPCError(w, http.StatusForbidden, req.ID, "policy denied", protocol.ErrorCodePolicyDenied, requestID, decision.DecisionID)
}

func buildForwardBody(req rpcRequest, params toolCallParams) ([]byte, error) {
	out := rpcRequestOut{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  params,
	}
	if out.JSONRPC == "" {
		out.JSONRPC = "2.0"
	}
	return json.Marshal(out)
}

func buildInvocationMeta(args map[string]interface{}, schemaVersion string) *invocationMeta {
	if len(args) == 0 && schemaVersion == "" {
		return nil
	}

	meta := &invocationMeta{ArgSchemaVersion: schemaVersion}
	if len(args) == 0 {
		return meta
	}

	b, err := json.Marshal(args)
	if err != nil {
		return meta
	}
	meta.ArgSizeBytes = len(b)
	meta.ContentHash = receipts.HashBytes(b)
	return meta
}

func writeInvocationReceipt(ctx context.Context, logger *slog.Logger, store invocationStore, tenant uuid.UUID, decisionID *uuid.UUID, requestID string, actor actorIdentity, mcpCtx protocol.MCPContext, policyHash string, policyVersion int, outcome string, statusCode *int, pdpLatency, toolLatency, totalLatency int, meta *invocationMeta, started time.Time, pepMode, enforcement, pdpStatus, traceID, spanID string) {
	if store == nil {
		logger.Warn("invocation receipt skipped (no store)")
		return
	}

	rb := invocationReceiptBody{
		ActorID:        actor.ID,
		ActorType:      actor.Type,
		ActorRoles:     actor.Roles,
		ActorSource:    actor.Source,
		ActorRoleCount: len(actor.Roles),
		MCPServer:      mcpCtx.Server,
		MCPTool:        mcpCtx.Tool,
		MCPMethod:      mcpCtx.Method,
		Workspace:      mcpCtx.Workspace,
		PolicyHash:     policyHash,
		PolicyVersion:  policyVersion,
		Outcome:        outcome,
		StatusCode:     statusCode,
		PDPLatencyMs:   pdpLatency,
		ToolLatencyMs:  toolLatency,
		TotalLatencyMs: totalLatency,
		Meta:           meta,
		StartedAt:      started.UTC().Format(time.RFC3339),
		RequestID:      requestID,
		PEPMode:        pepMode,
		Enforcement:    enforcement,
		PDPStatus:      pdpStatus,
	}

	bodyBytes, err := receipts.CanonicalJSON(rb)
	if err != nil {
		logger.Error("receipt canonical json failed", "err", err)
		return
	}

	prev, err := store.LastInvocationHash(ctx, tenant)
	if err != nil {
		logger.Error("receipt prev hash lookup failed", "err", err)
		return
	}

	hash := receipts.HashBytes(append([]byte(prev), bodyBytes...))
	if err := store.InsertInvocationReceipt(ctx, tenant, decisionID, requestID, toolIdentifier(mcpCtx.Server, mcpCtx.Tool), mcpCtx.Method, mcpCtx.Tool, outcome, statusCode, totalLatency, bodyBytes, prev, hash, traceID, spanID); err != nil {
		logger.Error("receipt insert failed", "err", err)
	}
}

func toolEndpoint(serverName, toolName string) string {
	if serverName == "" {
		return toolName
	}
	return serverName + "/" + toolName
}

func toolIdentifier(serverName, toolName string) string {
	if serverName == "" {
		return toolName
	}
	return serverName + ":" + toolName
}

func writeRPCError(w http.ResponseWriter, status int, id json.RawMessage, message, code, requestID string, decisionID ...string) {
	if strings.TrimSpace(requestID) == "" {
		requestID = uuid.NewString()
	}
	data := rpcErrorData{
		Error:     rpcErrorBody{Code: code, Message: message},
		RequestID: requestID,
	}
	if len(decisionID) > 0 && decisionID[0] != "" {
		data.DecisionID = decisionID[0]
	}

	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    -32000,
			Message: message,
			Data:    data,
		},
	}
	w.Header().Set("content-type", "application/json")
	w.Header().Set("x-request-id", requestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeRPCBadRequest(w http.ResponseWriter, id json.RawMessage, message, requestID string) {
	writeRPCError(w, http.StatusBadRequest, id, message, protocol.ErrorCodeBadRequest, requestID)
}

func writeRPCMethodNotAllowed(w http.ResponseWriter) {
	writeRPCError(w, http.StatusMethodNotAllowed, nil, "method not allowed", protocol.ErrorCodeMethodNotAllowed, "")
}

func writeRPCMethodNotSupported(w http.ResponseWriter, id json.RawMessage) {
	writeRPCError(w, http.StatusBadRequest, id, "method not supported", protocol.ErrorCodeMethodNotSupported, "")
}

func durationFromEnv(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	ms, err := time.ParseDuration(val + "ms")
	if err != nil {
		return def
	}
	return ms
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func intPtr(v int) *int { return &v }

func getenv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func normalizeIdentifier(value string, maxLen int, lower bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if lower {
		trimmed = strings.ToLower(trimmed)
	}
	if len(trimmed) > maxLen {
		return "", fmt.Errorf("identifier too long")
	}
	return trimmed, nil
}

func normalizeServerIdentifier(value string, maxLen int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if strings.Contains(trimmed, "://") {
		if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
			trimmed = parsed.Host
		}
	}
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.ToLower(strings.TrimSpace(trimmed))
	if len(trimmed) > maxLen {
		return "", fmt.Errorf("server identifier too long")
	}
	return trimmed, nil
}

func resolveActor(r *http.Request, params toolCallParams, def actorIdentity) (actorIdentity, error) {
	actor := def
	actorSourceHint := ""
	hasExplicit := false

	if params.Actor != nil {
		hasExplicit = true
		if params.Actor.ID != "" {
			actor.ID = params.Actor.ID
		}
		if params.Actor.Type != "" {
			actor.Type = params.Actor.Type
		}
		if params.Actor.Source != "" {
			actorSourceHint = params.Actor.Source
		} else {
			actorSourceHint = "mcp"
		}
		if params.Actor.Roles != nil {
			actor.Roles = params.Actor.Roles
		} else if hasExplicit {
			actor.Roles = []string{}
		}
	} else {
		headerID := strings.TrimSpace(r.Header.Get("x-umbra-actor-id"))
		headerType := strings.TrimSpace(r.Header.Get("x-umbra-actor-type"))
		headerRoles := parseCSV(r.Header.Get("x-umbra-actor-roles"))
		headerSource := strings.TrimSpace(r.Header.Get("x-umbra-actor-source"))
		if headerID != "" || headerType != "" || len(headerRoles) > 0 || headerSource != "" {
			hasExplicit = true
			if headerID != "" {
				actor.ID = headerID
			}
			if headerType != "" {
				actor.Type = headerType
			}
			if headerSource != "" {
				actorSourceHint = headerSource
			} else {
				actorSourceHint = "header"
			}
			if len(headerRoles) > 0 {
				actor.Roles = headerRoles
			} else {
				actor.Roles = []string{}
			}
		}
	}

	if actor.ID != "" && len(actor.ID) > maxActorIDLen {
		return actorIdentity{}, fmt.Errorf("actor id too long")
	}
	if actor.Type != "" && len(actor.Type) > maxActorTypeLen {
		return actorIdentity{}, fmt.Errorf("actor type too long")
	}
	if actorSourceHint != "" && len(actorSourceHint) > maxActorSourceLen {
		return actorIdentity{}, fmt.Errorf("actor source too long")
	}

	if actor.ID == "" {
		actor.ID = def.ID
	}

	actor.Type = normalizeActorType(actor.Type, def.Type)
	actor.Source = normalizeActorSource(actorSourceHint, def.Source, hasExplicit)

	if len(actor.ID) > maxActorIDLen {
		return actorIdentity{}, fmt.Errorf("actor id too long")
	}
	if len(actor.Type) > maxActorTypeLen {
		return actorIdentity{}, fmt.Errorf("actor type too long")
	}
	if len(actor.Source) > maxActorSourceLen {
		return actorIdentity{}, fmt.Errorf("actor source too long")
	}

	roles, err := sanitizeRoles(actor.Roles, hasExplicit)
	if err != nil {
		return actorIdentity{}, err
	}
	actor.Roles = roles

	if actor.Roles == nil {
		actor.Roles = []string{}
	}

	return actor, nil
}

func normalizeActorType(value, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		normalized = strings.ToLower(strings.TrimSpace(fallback))
	}
	if normalized == "user" {
		return "human"
	}
	if normalized == "" {
		return "agent"
	}
	return normalized
}

func normalizeActorSource(value, fallback string, explicit bool) string {
	if value != "" {
		return strings.ToLower(strings.TrimSpace(value))
	}
	if explicit {
		return "mcp"
	}
	return strings.ToLower(strings.TrimSpace(fallback))
}

func sanitizeRoles(roles []string, explicit bool) ([]string, error) {
	if roles == nil {
		if explicit {
			return []string{}, nil
		}
		return []string{}, nil
	}
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		val := strings.TrimSpace(role)
		if val == "" {
			continue
		}
		if len(val) > maxRoleLen {
			return nil, fmt.Errorf("role too long")
		}
		out = append(out, val)
		if len(out) > maxRoleCount {
			return nil, fmt.Errorf("too many roles")
		}
	}
	return out, nil
}
