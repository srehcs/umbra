package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
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
	dbstore "github.com/umbra-labs/agent-identity-control-plane/services/pep-gateway/internal/storage"
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

type demoInvokeRequest struct {
	Tool   string `json:"tool"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Actor  struct {
		ID    string   `json:"id"`
		Roles []string `json:"roles"`
	} `json:"actor"`
}

type invocationReceiptBody struct {
	Tool       string            `json:"tool"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Outcome    string            `json:"outcome"`
	StatusCode *int              `json:"status_code,omitempty"`
	LatencyMs  int               `json:"latency_ms"`
	Meta       map[string]string `json:"meta,omitempty"`
	StartedAt  string            `json:"started_at"` // RFC3339
}

func registerV0(mux *http.ServeMux, logger *slog.Logger) {
	tracer := otel.Tracer("umbra.pep")

	pdp := &PDPClient{
		BaseURL: getenv("PDP_URL", "http://pdp:8081"),
		Client:  &http.Client{Timeout: 3 * time.Second},
	}

	db, err := stor.Connect(getenv("DATABASE_URL", "postgres://umbra:umbra@postgres:5432/umbra?sslmode=disable"))
	if err != nil {
		logger.Error("db connect failed", "err", err)
	}
	store := dbstore.New(db)

	upstreamURL, err := url.Parse(getenv("UPSTREAM_URL", "http://upstream-sample:8090"))
	if err != nil {
		logger.Error("invalid upstream url", "err", err)
		upstreamURL = &url.URL{Scheme: "http", Host: "upstream-sample:8090"}
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// V0 convenience endpoint: POST /demo (JSON) to exercise the enforcement flow.
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var in demoInvokeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Translate to tool proxy call.
		r2 := r.Clone(r.Context())
		r2.Method = strings.ToUpper(strings.TrimSpace(in.Method))
		if r2.Method == "" {
			r2.Method = http.MethodGet
		}
		r2.URL.Path = "/tool" + in.Path

		// Attach actor hints.
		r2.Header.Set("x-umbra-tool-name", in.Tool)
		r2.Header.Set("x-umbra-actor-id", in.Actor.ID)
		r2.Header.Set("x-umbra-actor-roles", strings.Join(in.Actor.Roles, ","))

		handleToolProxy(tracer, logger, store, pdp, proxy)(w, r2)
	})

	// Primary V0 enforcement surface: /tool/* forwarded to an upstream, with delegated PDP decision.
	mux.Handle("/tool/", handleToolProxy(tracer, logger, store, pdp, proxy))
}

// handleToolProxy enforces a PDP decision before proxying to upstream.
func handleToolProxy(tracer trace.Tracer, logger *slog.Logger, store *dbstore.Store, pdp *PDPClient, proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("x-umbra-tenant-id")
		tenantID, err := uuid.Parse(tenantIDStr)
		if err != nil || tenantID == uuid.Nil {
			http.Error(w, "missing/invalid x-umbra-tenant-id", http.StatusBadRequest)
			return
		}

		reqID := r.Header.Get("x-umbra-request-id")
		if reqID == "" {
			reqID = uuid.NewString()
			r.Header.Set("x-umbra-request-id", reqID)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
		defer cancel()

		ctx, span := tracer.Start(ctx, "pep.enforce")
		defer span.End()

		toolName := strings.TrimSpace(r.Header.Get("x-umbra-tool-name"))
		if toolName == "" {
			toolName = "demo.http"
		}

		actorID := strings.TrimSpace(r.Header.Get("x-umbra-actor-id"))
		if actorID == "" {
			actorID = "user-1"
		}
		roles := parseCSV(r.Header.Get("x-umbra-actor-roles"))
		if len(roles) == 0 {
			roles = []string{"developer"}
		}

		// The upstream path we proxy (strip /tool prefix).
		upath := strings.TrimPrefix(r.URL.Path, "/tool")
		if upath == "" {
			upath = "/"
		}

		sc := trace.SpanContextFromContext(ctx)
		var traceCtx *protocol.TraceContext
		if sc.IsValid() {
			traceCtx = &protocol.TraceContext{
				RequestID: reqID,
				TraceID:   sc.TraceID().String(),
				SpanID:    sc.SpanID().String(),
			}
		} else {
			traceCtx = &protocol.TraceContext{RequestID: reqID}
		}

		payload := protocol.DecisionRequest{
			Tenant: protocol.TenantContext{TenantID: tenantID.String()},
			Actor:  protocol.Actor{Type: "user", ID: actorID, Roles: roles},
			Tool: protocol.Tool{
				Name:     toolName,
				Method:   r.Method,
				Endpoint: upath,
			},
			Trace: traceCtx,
		}

		span.SetAttributes(
			attribute.String("umbra.tenant_id", tenantID.String()),
			attribute.String("umbra.actor_id", actorID),
			attribute.String("umbra.tool", toolName),
			attribute.String("http.method", r.Method),
			attribute.String("http.route", upath),
		)

		started := time.Now()
		decision, status, err := pdp.Decide(ctx, payload)

		if err != nil {
			logger.Error("pdp decide failed", "err", err, "status", status)
			writeInvocationReceipt(ctx, logger, store, tenantID, nil, toolName, r.Method, upath, "error", intPtr(http.StatusServiceUnavailable), int(time.Since(started).Milliseconds()),
				map[string]string{"stage": "pdp"}, started)
			http.Error(w, "pdp unavailable", http.StatusServiceUnavailable)
			return
		}

		decisionID, _ := uuid.Parse(decision.DecisionID)
		if strings.ToLower(decision.Decision) != "allow" {
			writeInvocationReceipt(ctx, logger, store, tenantID, &decisionID, toolName, r.Method, upath, "denied", intPtr(http.StatusForbidden), int(time.Since(started).Milliseconds()),
				map[string]string{"reason": decision.Reason}, started)
			http.Error(w, "denied", http.StatusForbidden)
			return
		}

		// Allowed: proxy to upstream
		r.URL.Path = upath
		r.Host = "" // let reverse proxy set Host appropriately
		proxy.ModifyResponse = func(resp *http.Response) error {
			lat := int(time.Since(started).Milliseconds())
			writeInvocationReceipt(ctx, logger, store, tenantID, &decisionID, toolName, r.Method, upath, "success", intPtr(resp.StatusCode), lat,
				map[string]string{"upstream": proxyURLHost(proxy)}, started)
			return nil
		}
		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
			lat := int(time.Since(started).Milliseconds())
			logger.Error("upstream error", "err", e)
			writeInvocationReceipt(ctx, logger, store, tenantID, &decisionID, toolName, r.Method, upath, "error", intPtr(http.StatusBadGateway), lat,
				map[string]string{"upstream": proxyURLHost(proxy)}, started)
			http.Error(rw, "upstream error", http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	})
}

func writeInvocationReceipt(ctx context.Context, logger *slog.Logger, store *dbstore.Store, tenant uuid.UUID, decisionID *uuid.UUID,
	toolName, method, path, outcome string, statusCode *int, latencyMs int, meta map[string]string, started time.Time) {

	if store == nil {
		logger.Warn("invocation receipt skipped (no store)")
		return
	}

	rb := invocationReceiptBody{
		Tool:       toolName,
		Method:     method,
		Path:       path,
		Outcome:    outcome,
		StatusCode: statusCode,
		LatencyMs:  latencyMs,
		Meta:       meta,
		StartedAt:  started.UTC().Format(time.RFC3339),
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
	sc := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if sc.IsValid() {
		traceID, spanID = sc.TraceID().String(), sc.SpanID().String()
	}

	if err := store.InsertInvocationReceipt(ctx, tenant, decisionID, toolName, method, path, outcome, statusCode, latencyMs, bodyBytes, prev, hash, traceID, spanID); err != nil {
		logger.Error("receipt insert failed", "err", err)
	}
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

func proxyURLHost(_ *httputil.ReverseProxy) string {
	// Best-effort: keep it simple in V0
	return getenv("UPSTREAM_URL", "http://upstream-sample:8090")
}

func intPtr(v int) *int { return &v }

func getenv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
