use axum::{
    extract::{DefaultBodyLimit, State},
    http::StatusCode,
    response::{IntoResponse, Response},
    routing::{get, post},
    Json, Router,
};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    net::SocketAddr,
    time::{Duration, Instant},
};
use tokio::net::TcpListener;
use tower::limit::ConcurrencyLimitLayer;
use tower::ServiceBuilder;
use tracing::{error, info};
use uuid::Uuid;

#[derive(Clone)]
struct AppState {
    cfg: Config,
    pdp_client: Client,
    upstream_client: Client,
    controlplane_client: Client,
}

#[derive(Clone)]
struct Config {
    pep_mode: PepMode,
    pdp_url: String,
    controlplane_url: String,
    upstream_url: String,
    tenant_id: String,
    actor_id: String,
    actor_type: String,
    actor_roles: Vec<String>,
    server_name: String,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum PepMode {
    Enforce,
    Observe,
}

impl PepMode {
    fn from_env() -> Self {
        match std::env::var("PEP_MODE")
            .unwrap_or_else(|_| "observe".to_string())
            .to_lowercase()
            .as_str()
        {
            "enforce" => PepMode::Enforce,
            _ => PepMode::Observe,
        }
    }
}

#[derive(Deserialize)]
struct RpcRequest {
    jsonrpc: String,
    id: Option<Value>,
    method: String,
    params: Option<Value>,
}

#[derive(Serialize)]
struct RpcResponse {
    jsonrpc: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    id: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<RpcError>,
}

#[derive(Serialize)]
struct RpcError {
    code: i32,
    message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    data: Option<Value>,
}

#[derive(Deserialize, Serialize)]
struct ToolCallParams {
    name: String,
    #[serde(default)]
    server: Option<String>,
    #[serde(default)]
    request_id: Option<String>,
    #[serde(default)]
    workspace: Option<String>,
    #[serde(default)]
    args_schema_version: Option<String>,
    #[serde(default)]
    actor: Option<ActorParams>,
    #[serde(default)]
    arguments: Option<Value>,
}

#[derive(Deserialize, Serialize, Clone)]
struct ActorParams {
    #[serde(default)]
    id: Option<String>,
    #[serde(default)]
    r#type: Option<String>,
    #[serde(default)]
    roles: Option<Vec<String>>,
    #[serde(default)]
    source: Option<String>,
}

#[derive(Serialize)]
struct DecisionRequest {
    tenant: TenantContext,
    actor: Actor,
    tool: Tool,
    #[serde(skip_serializing_if = "Option::is_none")]
    mcp: Option<McpContext>,
    #[serde(skip_serializing_if = "Option::is_none")]
    trace: Option<TraceContext>,
}

#[derive(Serialize)]
struct TenantContext {
    tenant_id: String,
}

#[derive(Serialize)]
struct Actor {
    r#type: String,
    id: String,
    roles: Vec<String>,
    source: String,
}

#[derive(Serialize)]
struct Tool {
    name: String,
    method: String,
    endpoint: String,
}

#[derive(Serialize)]
struct McpContext {
    server: String,
    tool: String,
    method: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    workspace: Option<String>,
}

#[derive(Serialize)]
struct TraceContext {
    request_id: String,
}

#[derive(Deserialize)]
struct DecisionResponse {
    decision: String,
    decision_id: String,
    #[serde(default)]
    policy_hash: String,
    #[serde(default)]
    policy_version: i32,
    #[serde(default)]
    reason: String,
}

#[derive(Serialize)]
struct ReceiptIngestRequest {
    kind: &'static str,
    request_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    decision_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    decision: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    policy_hash: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    policy_version: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    tool_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    method: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    path: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    outcome: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    status_code: Option<i32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    latency_ms: Option<i32>,
    body: Value,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter("info")
        .init();

    let cfg = Config {
        pep_mode: PepMode::from_env(),
        pdp_url: env("PDP_URL", "http://pdp:8081"),
        controlplane_url: env("CONTROLPLANE_API_URL", "http://controlplane-api:8080"),
        upstream_url: env("MCP_UPSTREAM_URL", ""),
        tenant_id: env("MCP_TENANT_ID", "00000000-0000-0000-0000-000000000001"),
        actor_id: env("MCP_ACTOR_ID", "user-1"),
        actor_type: env("MCP_ACTOR_TYPE", "agent"),
        actor_roles: parse_csv(&env("MCP_ACTOR_ROLES", "developer")),
        server_name: env("MCP_SERVER_NAME", "demo.mcp"),
    };

    let pdp_timeout = duration_ms("PDP_TIMEOUT_MS", 3000);
    let tool_timeout = duration_ms("TOOL_TIMEOUT_MS", 5000);
    let receipt_timeout = duration_ms("RECEIPT_TIMEOUT_MS", 3000);

    let state = AppState {
        cfg,
        pdp_client: Client::builder().timeout(pdp_timeout).build().unwrap(),
        upstream_client: Client::builder().timeout(tool_timeout).build().unwrap(),
        controlplane_client: Client::builder()
            .timeout(receipt_timeout)
            .build()
            .unwrap(),
    };

    let app = Router::new()
        .route("/healthz", get(healthz))
        .route("/mcp", post(handle_mcp))
        .layer(DefaultBodyLimit::max(1 << 20))
        .layer(ServiceBuilder::new().layer(ConcurrencyLimitLayer::new(100)))
        .with_state(state);

    let addr: SocketAddr = format!("0.0.0.0:{}", env("PORT", "8084"))
        .parse()
        .expect("valid addr");
    info!("starting pep-rust on {}", addr);
    let listener = TcpListener::bind(addr).await.expect("bind");
    axum::serve(listener, app).await.expect("server failed");
}

async fn healthz() -> impl IntoResponse {
    Json(json!({
        "ok": true,
        "service": "pep-rust",
        "time": chrono::Utc::now().to_rfc3339(),
    }))
}

async fn handle_mcp(State(state): State<AppState>, body: bytes::Bytes) -> Response {
    let req: RpcRequest = match serde_json::from_slice(&body) {
        Ok(req) => req,
        Err(_) => {
            return rpc_error(
                StatusCode::BAD_REQUEST,
                None,
                "invalid json",
                "BAD_REQUEST",
                "",
                None,
            )
        }
    };

    if req.method.trim() != "tools/call" {
        return rpc_error(
            StatusCode::BAD_REQUEST,
            req.id,
            "method not supported",
            "METHOD_NOT_SUPPORTED",
            "",
            None,
        );
    }

    let params: ToolCallParams = match req
        .params
        .clone()
        .and_then(|v| serde_json::from_value(v).ok())
    {
        Some(p) => p,
        None => {
            return rpc_error(
                StatusCode::BAD_REQUEST,
                req.id,
                "invalid params",
                "BAD_PARAMS",
                "",
                None,
            )
        }
    };

    let request_id = params
        .request_id
        .clone()
        .unwrap_or_else(|| Uuid::new_v4().to_string());

    let started = Instant::now();

    let actor = resolve_actor(&state.cfg, &params.actor);
    let tool_name = params.name.trim().to_string();
    if tool_name.is_empty() {
        return rpc_error(
            StatusCode::BAD_REQUEST,
            req.id,
            "missing tool name",
            "BAD_PARAMS",
            &request_id,
            None,
        );
    }

    let server_name = params
        .server
        .clone()
        .unwrap_or_else(|| state.cfg.server_name.clone());
    let mcp_method = req.method.to_lowercase();
    let endpoint = format!("{}/{}", server_name, tool_name);

    let decision_req = DecisionRequest {
        tenant: TenantContext {
            tenant_id: state.cfg.tenant_id.clone(),
        },
        actor: Actor {
            r#type: actor.r#type.clone(),
            id: actor.id.clone(),
            roles: actor.roles.clone(),
            source: actor.source.clone(),
        },
        tool: Tool {
            name: tool_name.clone(),
            method: mcp_method.clone(),
            endpoint: endpoint.clone(),
        },
        mcp: Some(McpContext {
            server: server_name.clone(),
            tool: tool_name.clone(),
            method: mcp_method.clone(),
            workspace: params.workspace.clone(),
        }),
        trace: Some(TraceContext {
            request_id: request_id.clone(),
        }),
    };

    let pdp_started = Instant::now();
    let mut pdp_status = "ok".to_string();
    let mut decision: Option<DecisionResponse> = None;

    let pdp_url = format!("{}/v1/decision", state.cfg.pdp_url.trim_end_matches('/'));
    match state.pdp_client.post(pdp_url).json(&decision_req).send().await {
        Ok(resp) => match resp.json::<DecisionResponse>().await {
            Ok(dec) => decision = Some(dec),
            Err(_err) => {
                pdp_status = "unavailable".to_string();
            }
        },
        Err(_err) => {
            pdp_status = "unavailable".to_string();
        }
    }

    let pdp_latency_ms = pdp_started.elapsed().as_millis() as i32;
    let mut forward = false;
    let mut enforcement = "blocked";
    let mut outcome = "denied";
    let mut status = StatusCode::FORBIDDEN;
    let mut decision_id = None;
    let mut policy_hash = None;
    let mut policy_version = None;

    if let Some(dec) = &decision {
        decision_id = Some(dec.decision_id.clone());
        if !dec.policy_hash.is_empty() {
            policy_hash = Some(dec.policy_hash.clone());
        }
        if dec.policy_version > 0 {
            policy_version = Some(dec.policy_version);
        }
        if dec.decision.to_lowercase() == "allow" {
            forward = true;
            enforcement = "forwarded";
            outcome = "success";
            status = StatusCode::OK;
        } else if state.cfg.pep_mode == PepMode::Observe {
            forward = true;
            enforcement = "forwarded";
            outcome = "denied";
            status = StatusCode::OK;
        }
    } else if state.cfg.pep_mode == PepMode::Observe {
        forward = true;
        enforcement = "forwarded";
        outcome = "success";
        status = StatusCode::OK;
    } else {
        status = StatusCode::SERVICE_UNAVAILABLE;
    }

    if forward {
        let mut params_obj = serde_json::to_value(&params).unwrap_or_else(|_| json!({}));
        if let Value::Object(ref mut map) = params_obj {
            map.insert("request_id".to_string(), Value::String(request_id.clone()));
        }
        let forward_body = json!({
            "jsonrpc": req.jsonrpc,
            "id": req.id,
            "method": req.method,
            "params": params_obj,
        });

        let upstream_url = state.cfg.upstream_url.clone();
        if upstream_url.is_empty() {
            write_receipt(
                &state,
                &request_id,
                &tool_name,
                &mcp_method,
                &outcome,
                Some(status.as_u16() as i32),
                &decision_id,
                &policy_hash,
                policy_version,
                &actor,
                &server_name,
                params.workspace.clone(),
                enforcement,
                &pdp_status,
                decision.as_ref().map(|d| d.decision.as_str()),
                pdp_latency_ms,
                0,
                started.elapsed().as_millis() as i32,
            )
            .await;
            return rpc_error(
                StatusCode::BAD_GATEWAY,
                req.id,
                "upstream unavailable",
                "UPSTREAM_UNAVAILABLE",
                &request_id,
                decision_id.clone(),
            );
        }

        let tool_started = Instant::now();
        let upstream_resp = state
            .upstream_client
            .post(upstream_url)
            .header("content-type", "application/json")
            .json(&forward_body)
            .send()
            .await;

        match upstream_resp {
            Ok(resp) => {
                let status_code = resp.status();
                let body = resp.json::<Value>().await.unwrap_or_else(|_| json!({}));
                let mut final_outcome = outcome;
                if final_outcome == "success" && status_code.as_u16() >= 400 {
                    final_outcome = "error";
                }
                let tool_latency_ms = tool_started.elapsed().as_millis() as i32;
                let total_latency_ms = started.elapsed().as_millis() as i32;
                write_receipt(
                    &state,
                    &request_id,
                    &tool_name,
                    &mcp_method,
                    &final_outcome,
                    Some(status_code.as_u16() as i32),
                    &decision_id,
                    &policy_hash,
                    policy_version,
                    &actor,
                    &server_name,
                    params.workspace.clone(),
                    enforcement,
                    &pdp_status,
                    decision.as_ref().map(|d| d.decision.as_str()),
                    pdp_latency_ms,
                    tool_latency_ms,
                    total_latency_ms,
                )
                .await;
                return (status_code, Json(body)).into_response();
            }
            Err(err) => {
                error!("upstream error: {}", err);
                let tool_latency_ms = tool_started.elapsed().as_millis() as i32;
                let total_latency_ms = started.elapsed().as_millis() as i32;
                write_receipt(
                    &state,
                    &request_id,
                    &tool_name,
                    &mcp_method,
                    "error",
                    Some(StatusCode::BAD_GATEWAY.as_u16() as i32),
                    &decision_id,
                    &policy_hash,
                    policy_version,
                    &actor,
                    &server_name,
                    params.workspace.clone(),
                    enforcement,
                    &pdp_status,
                    decision.as_ref().map(|d| d.decision.as_str()),
                    pdp_latency_ms,
                    tool_latency_ms,
                    total_latency_ms,
                )
                .await;
                return rpc_error(
                    StatusCode::BAD_GATEWAY,
                    req.id,
                    "upstream error",
                    "UPSTREAM_ERROR",
                    &request_id,
                    decision_id.clone(),
                );
            }
        }
    }

    let total_latency_ms = started.elapsed().as_millis() as i32;
    write_receipt(
        &state,
        &request_id,
        &tool_name,
        &mcp_method,
        &outcome,
        Some(status.as_u16() as i32),
        &decision_id,
        &policy_hash,
        policy_version,
        &actor,
        &server_name,
        params.workspace.clone(),
        enforcement,
        &pdp_status,
        decision.as_ref().map(|d| d.decision.as_str()),
        pdp_latency_ms,
        0,
        total_latency_ms,
    )
    .await;

    if decision.is_none() {
        return rpc_error(
            StatusCode::SERVICE_UNAVAILABLE,
            req.id,
            "policy unavailable",
            "POLICY_UNAVAILABLE",
            &request_id,
            None,
        );
    }

    rpc_error(
        StatusCode::FORBIDDEN,
        req.id,
        "policy denied",
        "POLICY_DENIED",
        &request_id,
        decision_id.clone(),
    )
}

async fn write_receipt(
    state: &AppState,
    request_id: &str,
    tool_name: &str,
    method: &str,
    outcome: &str,
    status_code: Option<i32>,
    decision_id: &Option<String>,
    policy_hash: &Option<String>,
    policy_version: Option<i32>,
    actor: &ActorIdentity,
    server_name: &str,
    workspace: Option<String>,
    enforcement: &str,
    pdp_status: &str,
    decision_result: Option<&str>,
    pdp_latency_ms: i32,
    tool_latency_ms: i32,
    total_latency_ms: i32,
) {
    let body = json!({
        "actor": {
            "id": actor.id,
            "type": actor.r#type,
            "roles": actor.roles,
            "source": actor.source,
        },
        "mcp": {
            "server": server_name,
            "tool": tool_name,
            "method": method,
            "workspace": workspace,
        },
        "decision": decision_result,
        "outcome": outcome,
        "enforcement": enforcement,
        "pep_mode": match state.cfg.pep_mode { PepMode::Enforce => "enforce", PepMode::Observe => "observe" },
        "pdp_status": pdp_status,
        "pdp_latency_ms": pdp_latency_ms,
        "tool_latency_ms": tool_latency_ms,
        "total_latency_ms": total_latency_ms,
        "policy_hash": policy_hash,
        "policy_version": policy_version,
        "request_id": request_id,
    });

    let payload = ReceiptIngestRequest {
        kind: "invocation",
        request_id: request_id.to_string(),
        decision_id: decision_id.clone(),
        decision: None,
        policy_hash: policy_hash.clone(),
        policy_version,
        tool_name: Some(tool_identifier(server_name, tool_name)),
        method: Some(method.to_string()),
        path: Some(tool_name.to_string()),
        outcome: Some(outcome.to_string()),
        status_code,
        latency_ms: Some(total_latency_ms),
        body,
    };

    let url = format!(
        "{}/v1/receipts",
        state.cfg.controlplane_url.trim_end_matches('/')
    );
    let resp = state
        .controlplane_client
        .post(url)
        .header("x-umbra-tenant-id", &state.cfg.tenant_id)
        .json(&payload)
        .send()
        .await;

    if let Err(err) = resp {
        error!("receipt ingest failed: {}", err);
    }
}

fn resolve_actor(cfg: &Config, params: &Option<ActorParams>) -> ActorIdentity {
    if let Some(p) = params {
        return ActorIdentity {
            id: p.id.clone().unwrap_or_else(|| cfg.actor_id.clone()),
            r#type: p.r#type.clone().unwrap_or_else(|| cfg.actor_type.clone()),
            roles: p.roles.clone().unwrap_or_else(|| cfg.actor_roles.clone()),
            source: p
                .source
                .clone()
                .unwrap_or_else(|| "params".to_string()),
        };
    }
    ActorIdentity {
        id: cfg.actor_id.clone(),
        r#type: cfg.actor_type.clone(),
        roles: cfg.actor_roles.clone(),
        source: "env".to_string(),
    }
}

#[derive(Clone)]
struct ActorIdentity {
    id: String,
    r#type: String,
    roles: Vec<String>,
    source: String,
}

fn rpc_error(
    status: StatusCode,
    id: Option<Value>,
    msg: &str,
    code: &str,
    request_id: &str,
    decision_id: Option<String>,
) -> Response {
    let mut data = serde_json::Map::new();
    data.insert("error_code".to_string(), Value::String(code.to_string()));
    if !request_id.is_empty() {
        data.insert("request_id".to_string(), Value::String(request_id.to_string()));
    }
    if let Some(decision_id) = decision_id {
        data.insert("decision_id".to_string(), Value::String(decision_id));
    }
    let resp = RpcResponse {
        jsonrpc: "2.0",
        id,
        result: None,
        error: Some(RpcError {
            code: -32000,
            message: msg.to_string(),
            data: Some(Value::Object(data)),
        }),
    };
    (status, Json(resp)).into_response()
}

fn tool_identifier(server: &str, tool: &str) -> String {
    if server.is_empty() {
        return tool.to_string();
    }
    format!("{}:{}", server, tool)
}

fn env(key: &str, default: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| default.to_string())
}

fn duration_ms(key: &str, default_ms: u64) -> Duration {
    let val = std::env::var(key).ok().and_then(|v| v.parse::<u64>().ok());
    Duration::from_millis(val.unwrap_or(default_ms))
}

fn parse_csv(s: &str) -> Vec<String> {
    s.split(',')
        .map(|v| v.trim())
        .filter(|v| !v.is_empty())
        .map(|v| v.to_string())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::{body::Body, http::Request, routing::post, Router};
    use http_body_util::BodyExt;
    use std::sync::{Arc, Mutex};
    use tokio::net::TcpListener;
    use tower::ServiceExt;

    async fn start_server(app: Router) -> String {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });
        format!("http://{}", addr)
    }

    fn test_state(
        pep_mode: PepMode,
        pdp_url: String,
        controlplane_url: String,
        upstream_url: String,
    ) -> AppState {
        AppState {
            cfg: Config {
                pep_mode,
                pdp_url,
                controlplane_url,
                upstream_url,
                tenant_id: "11111111-1111-1111-1111-111111111111".to_string(),
                actor_id: "user-1".to_string(),
                actor_type: "agent".to_string(),
                actor_roles: vec!["developer".to_string()],
                server_name: "demo.mcp".to_string(),
            },
            pdp_client: Client::builder().timeout(Duration::from_secs(2)).build().unwrap(),
            upstream_client: Client::builder().timeout(Duration::from_secs(2)).build().unwrap(),
            controlplane_client: Client::builder().timeout(Duration::from_secs(2)).build().unwrap(),
        }
    }

    async fn read_body(resp: axum::response::Response) -> Value {
        let bytes = resp.into_body().collect().await.unwrap().to_bytes();
        serde_json::from_slice(&bytes).unwrap()
    }

    #[tokio::test]
    async fn allow_forwards_and_writes_receipt() {
        let receipts: Arc<Mutex<Vec<Value>>> = Arc::new(Mutex::new(Vec::new()));
        let receipts_clone = receipts.clone();
        let controlplane = Router::new().route(
            "/v1/receipts",
            post(move |Json(payload): Json<Value>| {
                let receipts_clone = receipts_clone.clone();
                async move {
                    receipts_clone.lock().unwrap().push(payload);
                    (
                        StatusCode::CREATED,
                        Json(json!({"receipt_id":"r1","hash":"h","prev_hash":""})),
                    )
                }
            }),
        );

        let pdp = Router::new().route(
            "/v1/decision",
            post(|| async {
                Json(json!({
                    "decision":"allow",
                    "decision_id":"dec-allow",
                    "policy_hash":"hash",
                    "policy_version":1,
                    "reason":"ok"
                }))
            }),
        );

        let upstream = Router::new().route(
            "/mcp",
            post(|| async { Json(json!({"ok": true})) }),
        );

        let controlplane_url = start_server(controlplane).await;
        let pdp_url = start_server(pdp).await;
        let upstream_url = format!("{}/mcp", start_server(upstream).await);

        let state = test_state(PepMode::Enforce, pdp_url, controlplane_url, upstream_url);
        let app = Router::new()
            .route("/mcp", post(handle_mcp))
            .with_state(state);

        let payload = json!({
            "jsonrpc":"2.0",
            "id":1,
            "method":"tools/call",
            "params":{
                "name":"demo.tool",
                "arguments":{"k":"v"},
                "request_id":"req-1"
            }
        });

        let resp = app
            .oneshot(
                Request::builder()
                    .method("POST")
                    .uri("/mcp")
                    .header("content-type", "application/json")
                    .body(Body::from(payload.to_string()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(resp.status(), StatusCode::OK);

        let stored = receipts.lock().unwrap();
        assert_eq!(stored.len(), 1);
        let receipt = &stored[0];
        assert_eq!(receipt["outcome"], "success");
        assert_eq!(receipt["method"], "tools/call");
        assert_eq!(receipt["tool_name"], "demo.mcp:demo.tool");
        assert_eq!(receipt["decision_id"], "dec-allow");
        assert_eq!(receipt["body"]["enforcement"], "forwarded");
    }

    #[tokio::test]
    async fn deny_blocks_in_enforce_mode() {
        let receipts: Arc<Mutex<Vec<Value>>> = Arc::new(Mutex::new(Vec::new()));
        let receipts_clone = receipts.clone();
        let controlplane = Router::new().route(
            "/v1/receipts",
            post(move |Json(payload): Json<Value>| {
                let receipts_clone = receipts_clone.clone();
                async move {
                    receipts_clone.lock().unwrap().push(payload);
                    (
                        StatusCode::CREATED,
                        Json(json!({"receipt_id":"r1","hash":"h","prev_hash":""})),
                    )
                }
            }),
        );

        let pdp = Router::new().route(
            "/v1/decision",
            post(|| async {
                Json(json!({
                    "decision":"deny",
                    "decision_id":"dec-deny",
                    "policy_hash":"hash",
                    "policy_version":1,
                    "reason":"no"
                }))
            }),
        );

        let upstream = Router::new().route(
            "/mcp",
            post(|| async { Json(json!({"ok": true})) }),
        );

        let controlplane_url = start_server(controlplane).await;
        let pdp_url = start_server(pdp).await;
        let upstream_url = format!("{}/mcp", start_server(upstream).await);

        let state = test_state(PepMode::Enforce, pdp_url, controlplane_url, upstream_url);
        let app = Router::new()
            .route("/mcp", post(handle_mcp))
            .with_state(state);

        let payload = json!({
            "jsonrpc":"2.0",
            "id":1,
            "method":"tools/call",
            "params":{"name":"demo.tool","request_id":"req-2"}
        });

        let resp = app
            .oneshot(
                Request::builder()
                    .method("POST")
                    .uri("/mcp")
                    .header("content-type", "application/json")
                    .body(Body::from(payload.to_string()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(resp.status(), StatusCode::FORBIDDEN);
        let body = read_body(resp).await;
        assert_eq!(body["error"]["data"]["error_code"], "POLICY_DENIED");

        let stored = receipts.lock().unwrap();
        assert_eq!(stored.len(), 1);
        let receipt = &stored[0];
        assert_eq!(receipt["outcome"], "denied");
        assert_eq!(receipt["body"]["enforcement"], "blocked");
        assert_eq!(receipt["decision_id"], "dec-deny");
    }

    #[tokio::test]
    async fn pdp_unavailable_blocks_in_enforce_mode() {
        let receipts: Arc<Mutex<Vec<Value>>> = Arc::new(Mutex::new(Vec::new()));
        let receipts_clone = receipts.clone();
        let controlplane = Router::new().route(
            "/v1/receipts",
            post(move |Json(payload): Json<Value>| {
                let receipts_clone = receipts_clone.clone();
                async move {
                    receipts_clone.lock().unwrap().push(payload);
                    (
                        StatusCode::CREATED,
                        Json(json!({"receipt_id":"r1","hash":"h","prev_hash":""})),
                    )
                }
            }),
        );

        let pdp = Router::new().route(
            "/v1/decision",
            post(|| async { (StatusCode::INTERNAL_SERVER_ERROR, "boom") }),
        );

        let controlplane_url = start_server(controlplane).await;
        let pdp_url = start_server(pdp).await;

        let state = test_state(PepMode::Enforce, pdp_url, controlplane_url, "http://localhost:0".to_string());
        let app = Router::new()
            .route("/mcp", post(handle_mcp))
            .with_state(state);

        let payload = json!({
            "jsonrpc":"2.0",
            "id":1,
            "method":"tools/call",
            "params":{"name":"demo.tool","request_id":"req-3"}
        });

        let resp = app
            .oneshot(
                Request::builder()
                    .method("POST")
                    .uri("/mcp")
                    .header("content-type", "application/json")
                    .body(Body::from(payload.to_string()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(resp.status(), StatusCode::SERVICE_UNAVAILABLE);
        let body = read_body(resp).await;
        assert_eq!(body["error"]["data"]["error_code"], "POLICY_UNAVAILABLE");

        let stored = receipts.lock().unwrap();
        assert_eq!(stored.len(), 1);
        let receipt = &stored[0];
        assert_eq!(receipt["body"]["pdp_status"], "unavailable");
    }
}
