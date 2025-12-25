# Phased Plan – Agent Mesh Observatory & Governor

This document is the canonical map of phases, deliverables, and current status for the project.

Status legend:
- **HAVE** – Exists in a reasonably complete form.
- **PARTIAL** – Concept/content exists but not yet as a clean standalone asset.
- **MISSING** – Needs to be created.

Known PDFs already in the project:
- `agent_mesh_observatory_v0_spec.pdf` – V0 product/tech spec (updated pivot).
- `agent_mesh_market_analysis.pdf` – market & competitive analysis (updated pivot).
- `Pr-centric Voice Manager + Autonomous Agent Workflow (open Hands + Open Router).pdf` – build workflow manual (updated pivot).

---

## Pivot Update – Shadow AI Control Plane (Dec 25, 2025)

We are reframing the company around a **single, painful problem that exists today**:

> Organizations can’t enforce policy at the *moment of AI action* — when a user or agent is about to (a) upload sensitive data, or (b) trigger a real tool call with real blast radius.

Most “shadow AI” programs stop at **visibility + policy docs**. Most “agent platforms” stop at **a managed runtime**. The gap is a unified **action-boundary control plane** that spans both:

1) **Employee / ad-hoc GenAI usage (Shadow AI)**
   - Browser, desktop, IDE copilots, personal accounts
   - File uploads + copy/paste + prompts to consumer tools
   - No centralized identity, routing, or enforcement

2) **Sanctioned internal agents (Tool-enabled automation)**
   - Framework diversity (LangGraph, CrewAI, homegrown, etc.)
   - Tool surface area (SQL, Jira, GitHub, cloud APIs, payments)
   - Inconsistent enforcement, weak audit trails, painful IR

**Our wedge:** one policy model + one evidence model across *all* AI interactions (humans + agents).

---

## Product Modules (Target Architecture)

### A) Enforcement / Data Plane (PEPs)
- **User path PEP:** secure web gateway / forward proxy integration (optionally a browser extension for UX).
- **Agent path PEP:** SDK wrappers and/or a tool-call proxy for MCP/HTTP/RPC tools.
- **Credential broker:** mint short-lived, scoped credentials per high-risk action (break-glass + approvals).

### B) Control Plane
- **Policy Decision Point (PDP):** returns allow/deny/allow-with-constraints with rationale + obligations.
- **Policy store:** versioned, testable policy-as-code workflow.
- **Identity:** SSO user identity + device identity + workload/service identity + environment context.

### C) Telemetry + IR Plane
- Canonical event schema across user prompts/uploads and agent tool calls.
- Investigation UX: interaction graph, timeline, replay, evidence bundle export.

---

## Phase 1 – Problem, Positioning, and Wedge

**Goal:** lock the crisp narrative + wedge and make it hard to confuse with “just observability”.

### Deliverables
1. **Vision One-Pager** (`docs/vision.md`) – **MISSING**
   - One sentence wedge, why now, buyer persona, and V0 success definition.

2. **Category positioning** (`docs/positioning.md`) – **MISSING**
   - “SSE/CASB for AI actions” + “agent tool-call control plane” (unified primitives).

3. **Threat model** (`docs/threat_model.md`) – **MISSING**
   - Top abuse cases: data exfil, prompt injection -> tool abuse, privilege escalation, lateral movement.

4. **V0 Demo Definition** (`docs/v0_demo_definition.md`) – **MISSING**
   - A scripted demo that proves real enforcement + evidence export.

---

## Phase 2 – Policy Model + Integration Strategy

**Goal:** decide the exact policy surface and where enforcement happens.

### Deliverables
1. **Policy model spec** (`docs/policy_model.md`) – **MISSING**
   - Entities: identity (user/agent), tool, resource, action, parameters, env, time.
   - Decisions: allow/deny/allow-with-constraints; rationale codes; obligations.

2. **Canonical event schema v0** (`docs/specs/event_schema_v0.md`) – **MISSING**
   - Trace + span fields plus AI-specific fields (tool name, input/output summaries, redaction flags).

3. **Integration plan** (`docs/integrations.md`) – **MISSING**
   - V0 targets: (a) one “user path” integration, (b) one “agent path” integration, (c) one high-risk tool.

---

## Phase 3 – V0 Build (Working Product Slice)

**Goal:** ship a working, demoable system with real enforcement.

### Deliverables
1. **Ingest API + storage** – **MISSING**
   - Postgres tables: events, decisions, policy versions, investigations.

2. **PDP service** – **MISSING**
   - JSON policy rules first; deterministic evaluation; decision logging.

3. **PEP #1 (Agent tool wrapper/proxy)** – **MISSING**
   - Intercept tool call, call PDP, enforce constraints, emit events.

4. **PEP #2 (User path routing)** – **MISSING**
   - Minimal proxy/routing path for approved GenAI endpoints + logging.

5. **UI v0** – **MISSING**
   - Recent events view + basic graph + “investigation export” action.

---

## Phase 4 – Hardening + Enterprise Controls

**Goal:** make it safe, operable, and integratable in real orgs.

### Deliverables
- Policy-as-code workflows (CI tests, approvals, change control).
- Integrations with SSE/SIEM (Splunk/Sentinel), OpenTelemetry, and ticketing.
- Strong privacy defaults (summaries/hashes; optional raw content).
- Retention + encryption + customer-managed keys (where needed).

---

## Phase 5 – GTM + Design Partner Rollout

**Goal:** turn V0 into revenue with design partners.

### Deliverables
- Design partner agreement + success criteria.
- Pricing hypothesis (per seat + per agent/tool; or per protected interaction).
- Case studies: “blocked X” + “reduced investigation time by Y” + “routed shadow AI to approved tools”.

---

## Current Assets Status (as of Dec 25, 2025)

- **HAVE:** Updated V0 spec and market analysis PDFs.
- **HAVE:** Updated PR-centric build workflow manual.
- **PARTIAL/MISSING:** Most repo- and build-specific docs (policy model, event schema, integrations, threat model).
