package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
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

type PDPClient struct {
	BaseURL string
	Client  *http.Client
}

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

	body, _ := io.ReadAll(res.Body)
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

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
	Server    string                 `json:"server,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

type invocationMeta struct {
	ArgKeys  []string `json:"arg_keys,omitempty"`
	ArgCount int      `json:"arg_count,omitempty"`
}

type invocationReceiptBody struct {
	ToolServer     string          `json:"tool_server,omitempty"`
	ToolName       string          `json:"tool_name"`
	ToolMethod     string          `json:"tool_method"`
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
	actorID       string
	actorRoles    []string
	serverName    string
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
	roles := parseCSV(getenv("MCP_ACTOR_ROLES", "developer"))
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
		actorID:       actorID,
		actorRoles:    roles,
		serverName:    serverName,
	}

	mux.HandleFunc("/mcp", h.handleMCP)
}

func (h *mcpHandler) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, "invalid request", "BAD_REQUEST", "")
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPCError(w, http.StatusBadRequest, nil, "invalid json", "BAD_REQUEST", "")
		return
	}

	if strings.TrimSpace(req.Method) != "tools/call" {
		writeRPCError(w, http.StatusBadRequest, req.ID, "method not supported", "METHOD_NOT_SUPPORTED", "")
		return
	}

	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, http.StatusBadRequest, req.ID, "invalid params", "BAD_PARAMS", "")
		return
	}

	toolName := strings.TrimSpace(params.Name)
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

	serverName := strings.TrimSpace(params.Server)
	if serverName == "" {
		serverName = h.serverName
	}

	meta := buildInvocationMeta(params.Arguments)

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

	logger := h.logger.With("request_id", requestID)
	if traceID != "" {
		logger = logger.With("trace_id", traceID, "span_id", spanID)
	}

	span.SetAttributes(
		attribute.String("umbra.tenant_id", tenantID.String()),
		attribute.String("umbra.request_id", requestID),
		attribute.String("mcp.method", req.Method),
		attribute.String("mcp.tool", toolName),
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
		Actor:  protocol.Actor{Type: "user", ID: h.actorID, Roles: h.actorRoles},
		Tool: protocol.Tool{
			Name:     toolName,
			Method:   req.Method,
			Endpoint: toolEndpoint(serverName, toolName),
		},
		Trace: traceCtx,
	}

	decision, status, err := h.pdp.Decide(ctx, decisionReq)
	pdpLatency := int(time.Since(pdpStarted).Milliseconds())
	pdpStatus := "ok"

	var decisionID *uuid.UUID
	if err == nil {
		if parsed, err := uuid.Parse(decision.DecisionID); err == nil {
			decisionID = &parsed
		}
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
	var toolBody []byte

	if forward {
		forwardBody, err := buildForwardBody(req, params)
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, serverName, toolName, req.Method, outcome, intPtr(http.StatusBadRequest), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadRequest, req.ID, "invalid request", "BAD_REQUEST", requestID)
			return
		}

		toolStarted := time.Now()
		toolReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.upstreamURL, bytes.NewReader(forwardBody))
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, serverName, toolName, req.Method, "error", intPtr(http.StatusBadGateway), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadGateway, req.ID, "upstream unavailable", "UPSTREAM_UNAVAILABLE", requestID)
			return
		}
		toolReq.Header.Set("content-type", "application/json")

		toolRes, err := h.toolClient.Do(toolReq)
		toolLatency = int(time.Since(toolStarted).Milliseconds())
		if err != nil {
			writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, serverName, toolName, req.Method, "error", intPtr(http.StatusBadGateway), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
			writeRPCError(w, http.StatusBadGateway, req.ID, "upstream error", "UPSTREAM_ERROR", requestID)
			return
		}
		defer toolRes.Body.Close()
		toolBody, _ = io.ReadAll(toolRes.Body)
		toolStatusVal := toolRes.StatusCode
		toolStatus = &toolStatusVal

		if outcome == "success" {
			if toolRes.StatusCode >= 400 {
				outcome = "error"
			}
		}
		writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, serverName, toolName, req.Method, outcome, toolStatus, pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(toolRes.StatusCode)
		_, _ = w.Write(toolBody)
		return
	}

	writeInvocationReceipt(ctx, logger, h.store, tenantID, decisionID, requestID, serverName, toolName, req.Method, outcome, intPtr(statusCode), pdpLatency, toolLatency, int(time.Since(started).Milliseconds()), meta, started, h.pepMode, enforcement, pdpStatus, traceID, spanID)

	if err != nil {
		writeRPCError(w, http.StatusServiceUnavailable, req.ID, "policy unavailable", "POLICY_UNAVAILABLE", requestID)
		return
	}

	writeRPCError(w, http.StatusForbidden, req.ID, "policy denied", "POLICY_DENIED", requestID, decision.DecisionID)
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

func buildInvocationMeta(args map[string]interface{}) *invocationMeta {
	if len(args) == 0 {
		return nil
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return &invocationMeta{ArgKeys: keys, ArgCount: len(keys)}
}

func writeInvocationReceipt(ctx context.Context, logger *slog.Logger, store invocationStore, tenant uuid.UUID, decisionID *uuid.UUID, requestID, serverName, toolName, method, outcome string, statusCode *int, pdpLatency, toolLatency, totalLatency int, meta *invocationMeta, started time.Time, pepMode, enforcement, pdpStatus, traceID, spanID string) {
	if store == nil {
		logger.Warn("invocation receipt skipped (no store)")
		return
	}

	rb := invocationReceiptBody{
		ToolServer:     serverName,
		ToolName:       toolName,
		ToolMethod:     method,
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
	if err := store.InsertInvocationReceipt(ctx, tenant, decisionID, requestID, toolIdentifier(serverName, toolName), method, toolName, outcome, statusCode, totalLatency, bodyBytes, prev, hash, traceID, spanID); err != nil {
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
	data := map[string]string{"error_code": code}
	if requestID != "" {
		data["request_id"] = requestID
	}
	if len(decisionID) > 0 && decisionID[0] != "" {
		data["decision_id"] = decisionID[0]
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
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
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
