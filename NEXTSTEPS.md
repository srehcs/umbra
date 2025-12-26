# Next Steps — Linear Backlog (Umbra Identity Control Plane V0-C)

This backlog is sequenced to reach an **MVP** aligned with the product vision:
a control plane that makes agent tool usage **policy-governed, enforceable, and auditable**.

## MVP strategy
- **MCP-first** enforcement (primary integration surface).
- **Scoped CLI wrapper** next (allowlisted commands only, strict redaction).
- **Passive discovery** after that (observe-only instrumentation to map agent/tool interactions before broad enforcement).

---

## Linear Project: V0 Demo Readiness
**Goal:** A repeatable, end-to-end demo: policy change → allow/deny decision → receipts visible in UI, with integrity and correlation.

### Epic: Local demo reliability
**Issue: T-101 — Deterministic local demo**
- Description: Make `make dev` + `make seed` deterministic and idempotent.
- Acceptance:
  - Fresh clone → `make dev && make seed` → UI shows seeded tools/policies/receipts.
  - Re-running `make seed` produces no duplicates and no errors.
- Labels: `mvp`, `devex`, `reliability`
- Priority: P0

**Issue: T-102 — Observe vs enforce mode**
- Description: Add `PEP_MODE=observe|enforce`.
  - Observe: never blocks, records receipts.
  - Enforce: blocks on deny (default-deny posture).
- Acceptance: Switching env var changes runtime behavior without code changes.
- Labels: `mvp`, `pep`, `policy`
- Priority: P0

**Issue: T-103 — Standard error envelope**
- Description: Standardize service errors as `{ error: { code, message }, request_id }`.
- Acceptance: UI and API clients never receive raw stack traces; errors are consistent across services.
- Labels: `mvp`, `api`, `quality`
- Priority: P1

### Epic: Policy correctness and explainability
**Issue: T-111 — Server-side policy validation**
- Description: Validate policy payloads in the Control Plane API (schema + bounds).
- Acceptance: A policy that would break PDP cannot be created/activated.
- Labels: `mvp`, `policy`, `api`
- Priority: P0

**Issue: T-112 — Policy simulation endpoint**
- Description: Add an endpoint to evaluate a decision without invoking a tool.
- Acceptance: Given a sample request, returns decision + reason + matched rule.
- Labels: `mvp`, `policy`, `ux`
- Priority: P1

**Issue: T-113 — Decision explanations (“why”)**
- Description: Include `reason` and `rule_index` (or rule id) in DecisionResponse.
- Acceptance: Deny reasons are visible in UI and returned via API.
- Labels: `mvp`, `policy`, `observability`
- Priority: P1

### Epic: Receipts integrity and correlation
**Issue: T-121 — Receipt integrity verification**
- Description: Add `/v1/receipts/verify` (or a script) to validate receipt hash chain.
- Acceptance: Verification detects missing/altered links in the chain.
- Labels: `mvp`, `receipts`, `security`
- Priority: P0

**Issue: T-122 — Correlation requirements**
- Description: Ensure `decision_id`, `request_id`, `trace_id` propagate PEP → PDP → receipts.
- Acceptance: A single action can be traced end-to-end using receipt fields + OTel IDs.
- Labels: `mvp`, `otel`, `receipts`
- Priority: P0

---

## Linear Project: MCP-First Enforcement (MVP Integration)
**Goal:** MCP tool calls are governed by PDP decisions and produce receipts.

### Epic: MCP enforcement adapter
**Issue: T-201 — MCP adapter service (PEP)**
- Description: Build MCP adapter that intercepts `tools/call`, creates a `DecisionRequest`, enforces allow/deny, and emits receipts.
- Acceptance:
  - MCP call → PDP decision → allow/deny outcome.
  - Receipts include MCP surface metadata and correlation IDs.
- Labels: `mvp`, `mcp`, `pep`, `receipts`
- Priority: P0

**Issue: T-202 — MCP identity mapping**
- Description: Define mapping from MCP caller identity to `actor.id`, `actor.type`, and roles.
- Acceptance: Actor identity is consistent and visible in receipts and UI.
- Labels: `mvp`, `mcp`, `identity`
- Priority: P0

**Issue: T-203 — MCP context model**
- Description: Extend decision input to carry MCP-relevant attributes (server, tool name, method, workspace/repo if available). Redact tool args by default.
- Acceptance: Policies can key off MCP context without leaking secrets.
- Labels: `mvp`, `mcp`, `policy`, `security`
- Priority: P1

**Issue: T-204 — MCP demo integration**
- Description: Provide a working demo configuration with at least one MCP client + sample tool.
- Acceptance: Documented steps in `identity/control-plane/docs/04_playbook_demo.md` to run MCP demo locally.
- Labels: `mvp`, `mcp`, `demo`, `docs`
- Priority: P1

---

## Linear Project: Scoped CLI Wrapper (Post-MVP Iteration)
**Goal:** Add a limited-scope CLI wrapper for selected commands with strict redaction and receipts.

### Epic: CLI wrapper (allowlisted commands)
**Issue: T-301 — CLI wrapper MVP (allowlist only)**
- Description: Add wrapper for a small set of commands/tools; translate to decision requests and emit receipts.
- Acceptance: Allowlisted commands can be allowed/denied by policy; all executions produce receipts.
- Labels: `cli`, `pep`, `receipts`
- Priority: P1

**Issue: T-302 — Redaction rules for CLI**
- Description: Ensure secrets (tokens, env vars, sensitive flags) are never stored in receipts/logs.
- Acceptance: Tests prove redaction; receipts contain metadata-only.
- Labels: `cli`, `security`, `receipts`
- Priority: P0

---

## Linear Project: Passive Discovery (Post-MVP Iteration)
**Goal:** Observe agent/tool activity safely before enforcing widely.

### Epic: Observe-only instrumentation
**Issue: T-401 — Passive discovery mode**
- Description: Add an observe-only collector for tool invocations to build an inventory (tools, actions, frequencies, actors) without blocking.
- Acceptance: Produces records without enforcement; UI can display discovery summaries.
- Labels: `discovery`, `observability`
- Priority: P2

---

## Linear Project: Enterprise Auth (Keycloak/OIDC) — Documented “Path to Yes”
**Goal:** Move from dev headers to claim-based tenancy and RBAC.

### Epic: OIDC enablement
**Issue: T-501 — Keycloak dev realm + compose integration**
- Description: Add optional Keycloak to docker-compose and realm import.
- Acceptance: Local login works when auth is enabled.
- Labels: `auth`, `keycloak`, `devex`
- Priority: P2

**Issue: T-502 — API JWT validation middleware**
- Description: Validate JWTs; derive tenant + roles from claims. Header-based tenancy allowed only in dev mode behind a flag.
- Acceptance: All admin APIs require JWT when enabled; role enforcement works.
- Labels: `auth`, `security`, `api`
- Priority: P2

---

## Recommended implementation order (short)
1) T-101, T-102, T-111, T-121, T-122  
2) T-201, T-202, T-203  
3) T-112, T-113, T-204  
4) CLI wrapper (T-301, T-302)  
5) Passive discovery (T-401)  
6) Keycloak/OIDC (T-501, T-502)
