# Phased Plan – Agent Mesh Observatory & Governor

This document is the canonical map of phases, deliverables, and current status for the project.

Status legend:
- **HAVE** – Exists in a reasonably complete form.
- **PARTIAL** – Concept/content exists (e.g., in chat, PDFs, or rough notes) but not yet as a clean standalone asset.
- **MISSING** – Needs to be created.

Known PDFs already in the project:
- `agent_mesh_observatory_v0_spec.pdf` – original product/tech spec (user-uploaded).
- `agent_mesh_market_analysis.pdf` – McKinsey/Bain-style market & competitive analysis (generated from prior answer).

---

## Pivot Update – Security Control Plane + Agent IR/Forensics (Dec 23, 2025)

We are **expanding the wedge** beyond “observability + basic guardrails” into two crisp, monetizable problems:

1) **Agent security control plane for tool/skill permissions (PDP/PEP + audit)**
   - Fine‑grained policy: *agents can call these tools, on these resources, with these parameters, in these environments*
   - Enforcement: consistent allow/deny across **multiple agent runtimes** (vendor‑agnostic)
   - Evidence: versioned policy changes + per‑decision audit log

2) **Agent incident response + forensics (“flight recorder”)**
   - Postmortem-grade incident timeline across traces/runtimes
   - Path replay + “why did this happen” evidence bundle export (graph snapshot, key events, metrics, policy context)

**Important:** The original V0 spec treated “semantic allow/deny decisions” as a non-goal. With this pivot, semantic allow/deny becomes a **core V0 capability**, so the spec has been updated accordingly.

---

## Phase 1 – Idea & Vision

**Goal:** Anchor *why* this exists, *who* it is for, and *what* v0 success means.

### Deliverables

1. **Vision One-Pager**  
   - **File (proposed):** `docs/vision.md`  
   - **Status:** PARTIAL  
   - **Purpose:**  
     - Articulate the problem: unobservable, ungoverned agent/skill meshes causing loops, cost storms, and risk.  
     - Articulate the solution: a **neutral runtime mesh** (observatory + governor) across agents, skills, tools, and vendors.  
     - Define primary user/buyer (AI platform / infra teams) and “why now.”  
   - **Existing sources:**  
     - `agent_mesh_observatory_v0_spec.pdf` (initial product framing).  
     - `agent_mesh_market_analysis.pdf` (market context and positioning).  

2. **Product Tenets / Principles**  
   - **File (proposed):** `docs/product_tenets.md`  
   - **Status:** PARTIAL  
   - **Purpose:** Capture non-negotiable design principles, including:  
     - Neutral & vendor-agnostic (multi-LLM, multi-framework, agents **and** Claude Skills).  
     - Plug-and-play (SDK + proxy → “time to first graph”).  
     - Telemetry-first (graph + metrics + anomalies).  
     - Guardrails as first-class (QPS, concurrency, kill switches).  
     - Secure & privacy-aware (TLS, redaction, self-hostable).  
     - Future-proof **node types** (agents, skills, tools, latent modules, environments, etc.).  
   - **Existing sources:**  
     - Prior conversation where these were enumerated.  
     - V0 spec and market analysis for supporting detail.

3. **v0 Demo Definition**  
   - **File (proposed):** `docs/v0_demo_definition.md`  
   - **Status:** MISSING  
   - **Purpose:** Define “what counts as a first real win”:  
     - Integrate SDK/proxy with one or more agents/skills.  
     - Render a live mesh graph of agents/skills/tools.  
     - Show per-node metrics: calls, latency, error rate, cost.  
     - Detect at least one class of pathology (e.g., loop or QPS spike).  
     - Allow flipping a kill switch on a node and observe traffic stopping.  
   - **Existing sources:**  
     - Described verbally in prior discussion; needs to be crystallized.

4. **Market & Competitor Summary (Lite)**  
   - **File (proposed):** `docs/market_summary.md`  
   - **Status:** PARTIAL  
   - **Purpose:** Condensed 1–2 page view of the market and competitors:  
     - Categories: LLM observability, agent mesh/gateways, AI governance, APM/DIY.  
     - For each, 2–3 bullets on strengths and gaps.  
   - **Existing sources:**  
     - `agent_mesh_market_analysis.pdf` (full, long-form version).  

5. **Security Control Plane: Product Narrative + Policy Surface (Wedge v0)**
   - **File (proposed):** `docs/security_control_plane.md`
   - **Status:** PARTIAL
   - **Purpose:**
     - Define the *policy surface* for tool/skill permissions (who/what/where/when/how).
     - Include example policies (MCP tools, SQL tools, web-browse tools).
     - Define the minimal enforcement strategy (SDK hooks + optional proxy).
   - **Notes:**
     - This explicitly *supersedes* the original V0 non-goal “No semantic allow/deny decisions.”

6. **Agent Incident Response & Forensics Narrative (“Flight Recorder”)**
   - **File (proposed):** `docs/agent_ir_forensics.md`
   - **Status:** PARTIAL
   - **Purpose:**
     - Define the incident workflow (detect → contain → explain → export evidence).
     - Specify what an “evidence bundle” includes (timeline, graph snapshot, key events, metrics, policy context).
     - Define “path replay” concept (reconstruct steps without needing full prompt content).

---

## Phase 2 – Prep & Viability

**Goal:** Ensure this is personally viable (time + money) and technically coherent before heavy building.

### Deliverables

1. **Personal Runway & Scenario Plan**  
   - **File (proposed):** `docs/personal_runway_and_scenarios.md` (+ simple spreadsheet)  
   - **Status:** MISSING  
   - **Purpose:**  
     - Document savings, burn, and time horizon.  
     - Define Scenario A (keep full-time job) vs Scenario B (later part-time/sabbatical).  
     - Set explicit *gates* for even considering quitting (e.g., design partners, paid pilots).  

2. **LLM & Infra Cost Model**  
   - **File (proposed):** `financials/llm_cost_model.xlsx`  
   - **Status:** MISSING  
   - **Purpose:**  
     - Model expected token usage and model mix (frontier vs OSS APIs).  
     - Produce low / medium / aggressive monthly spend scenarios.  
     - Set a working budget cap (e.g., $150–$250/month initially).  

3. **Tools & Stack Decisions Doc**  
   - **File (proposed):** `docs/stack_decisions.md`  
   - **Status:** PARTIAL  
   - **Purpose:** Record chosen stack to avoid churn:  
     - Backend language & framework.  
     - DBs (Postgres + optional graph/time-series).  
     - Frontend (React/Next/other).  
     - Infra (Docker, CI/CD via GitHub Actions).  
     - AI dev tools (Cursor/Copilot/etc.) and LLM providers (OpenAI + managed OSS).  

4. **Agent / Auto-Dev Guardrails**  
   - **File (proposed):** `docs/agent_contrib_rules.md`  
   - **Status:** MISSING  
   - **Purpose:** Rules for letting AI agents modify the repo safely:  
     - Monorepo only; no multi-repo sprawl initially.  
     - Branch-per-task; agents cannot merge to `main`.  
     - Directory fences per task (bounded blast radius).  
     - Max diff size / file count.  
     - CI must be green before human merge.  

7. **Threat Model + Abuse Cases (Prompt Injection + Tool Abuse)**
   - **File (proposed):** `docs/threat_model.md`
   - **Status:** MISSING
   - **Purpose:**
     - Enumerate top abuse cases (exfiltration, privilege escalation, destructive actions, lateral movement).
     - Map each to controls (policy, redaction, sandboxing, rate limits, approvals).
     - Define “must block” vs “detect & alert” categories.

8. **Policy Model & Evaluation Semantics**
   - **File (proposed):** `docs/policy_model.md`
   - **Status:** MISSING
   - **Purpose:**
     - Define identities (agent, tool, workload, environment) and the resource model.
     - Define decision outputs (allow/deny/allow-with-constraints) + rationale codes for audit.
     - Decide policy language (minimal JSON rules first; optional Rego/OPA compatibility later).

9. **Evidence, Retention, Privacy, and Encryption Plan**
   - **File (proposed):** `docs/evidence_retention_and_privacy.md`
   - **Status:** MISSING
   - **Purpose:**
     - What we store vs don’t store (summaries/hashes vs raw prompts).
     - Redaction model + PII handling.
     - Encryption at rest/in transit + key ownership assumptions for self-hosted customers.

---

## Phase 3 – Roadmap to v0 Working Demo

**Goal:** Translate vision + prep into a concrete path to a v0 demo (GitHub repo + running system).

### Deliverables

1. **Repo Structure Spec**  
   - **File (proposed):** `docs/repo_structure.md`  
   - **Status:** MISSING  
   - **Purpose:** Define monorepo layout and conventions:  
     - `/backend` – ingest API, processing, storage.  
     - `/frontend` – mesh UI and metrics dashboards.  
     - `/sdk` – language-specific instrumentation SDKs (e.g., `python/`, `typescript/`).  
     - `/infra` – Docker, compose, early k8s manifests.  
     - `/docs` – specs, product docs, GTM materials.  

2. **Telemetry & Event Schema v0**  
   - **File (proposed):** `docs/specs/telemetry_schema_v0.md`  
   - **Status:** MISSING  
   - **Purpose:** Define the canonical runtime event model, designed to survive agents → skills → latent modules:  
     - `actor_type`, `actor_id`, `target_type`, `target_id`.  
     - `channel` (http, mcp, internal, etc.) and `modality` (text, latent, image, etc.).  
     - `duration_ms`, `status`, `cost`, `metadata`, `redaction` flags.  
   - **Notes:**  
     - This is where Claude Skills, tools, latent modules, and classic “agents” are all modeled as node types and edges.  
     - v0 focuses on text/HTTP, but schema is modality-agnostic.

3. **v0 Feature Checklist & Roadmap**  
   - **File (proposed):** `docs/v0_feature_checklist.md`  
   - **Status:** MISSING  
   - **Purpose:** Checklist of v0 features with status and owner:  
     - Ingest endpoint(s) for telemetry events.  
     - Storage & aggregation for metrics.  
     - Mesh graph API and basic UI.  
     - At least one heuristic (loop or QPS/cost spike).  
     - Kill switches wired through SDK/proxy.  

4. **Agent Task Schema & Backlog**  
   - **Files (proposed):**  
     - `docs/agent_task_schema.md`  
     - `tasks/backlog.yaml`  
   - **Status:** MISSING  
   - **Purpose:** Formalize how you feed tasks to coding agents:  
     - Task schema: `id`, `area`, `goal`, `inputs`, `acceptance_criteria`.  
     - Initial backlog of 20–40 tasks covering scaffolding through v0 features.

5. **Minimal CI/CD Setup**  
   - **Files (proposed):**  
     - `.github/workflows/ci.yaml`  
     - `docs/ci_cd.md`  
   - **Status:** MISSING  
   - **Purpose:** Ensure every PR (human or agent) runs tests & lint; optional auto-deploy of demo env on merge to `main`.

6. **Policy Decision Point (PDP) Service – MVP**
   - **File (proposed):** `docs/pdp_service_spec.md`
   - **Status:** MISSING
   - **Purpose:**
     - API: `POST /v1/decide` → decision + rationale + obligations (e.g., “max_rows=100”, “read-only”).
     - Policy storage + versioning + rollout (dev/stage/prod).
     - First admin UI: policy editor + “test a decision” simulator.

7. **Policy Enforcement Points (PEPs) – First Connectors**
   - **File (proposed):** `docs/pep_connectors.md`
   - **Status:** MISSING
   - **Purpose:** Implement enforcement for the highest‑leverage tool types:
     - MCP tools
     - SQL tools (with query classification / parameter constraints)
     - Web-browse tools (domain allowlists, download blocks)
   - **Output:** “drop‑in” wrappers in the Python/TS SDKs + optional HTTP proxy mode.

8. **Forensics Pipeline + Evidence Bundle Export**
   - **File (proposed):** `docs/forensics_pipeline.md`
   - **Status:** MISSING
   - **Purpose:**
     - Incident timeline view (trace-centric + cross-trace correlation).
     - Path replay (reconstruct call graph + key inputs/outputs summaries).
     - One-click export: JSON + PDF/HTML bundle for postmortems.

9. **Golden Demo Scenarios (Security + IR)**
   - **File (proposed):** `docs/demo_scenarios.md`
   - **Status:** MISSING
   - **Purpose:** Scripted demos you can run locally:
     - Prompt injection attempt → blocked by tool policy
     - Excessive tool depth / loop → detected + kill-switched
     - Incident → export evidence bundle (timeline + policy context)

---

## Phase 4 – Product & Architecture Deep Dive

**Goal:** Turn “v0 runs” into a coherent, extensible product with clear internal structure.

### Deliverables

1. **System Architecture Overview**  
   - **File (proposed):** `docs/architecture/00_system_overview.md` (+ diagrams)  
   - **Status:** MISSING  
   - **Purpose:** High-level context, container, and component diagrams:  
     - Show users, your system, external LLM providers, agent/skill platforms.  
     - Show internal services (ingest, processor, UI, DBs, queues).  

2. **Component Tech Specs**  
   - **Files (proposed):**  
     - `docs/specs/ingest_and_processing.md`  
     - `docs/specs/storage_and_schema.md`  
     - `docs/specs/heuristics_and_anomalies.md`  
     - `docs/specs/guardrails_and_config.md`  
     - `docs/specs/ui_ux_mesh_and_metrics.md`  
   - **Status:** PARTIAL  
   - **Purpose:** For each major component, specify interfaces, data flow, and v0 vs v1+ scope.  
   - **Existing sources:**  
     - `agent_mesh_observatory_v0_spec.pdf` contains much of this in narrative form.

3. **Node Types & Extensibility (Agents, Skills, Tools, Latent Modules, etc.)**  
   - **File (proposed):** `docs/specs/node_types_and_extensibility.md`  
   - **Status:** MISSING  
   - **Purpose:** Codify how you treat different “things” in the mesh:  
     - Node types: `agent`, `skill`, `tool`, `system`, `latent_module`, `environment`, etc.  
     - Mapping to telemetry fields (`actor_type`, `target_type`, tags).  
     - How Claude Skills, future Skill APIs, and LatentMAS-style internals plug in via adapters.  
     - Guarantee that you are not locked to just “agents” as a concept.  

4. **Architecture Decision Records (ADRs)**  
   - **Files (proposed):** e.g.,  
     - `docs/adr/0001_backend_language.md`  
     - `docs/adr/0002_database_choice.md`  
     - `docs/adr/0003_monorepo_decision.md`  
   - **Status:** MISSING  
   - **Purpose:** Capture key architectural choices and rationale to avoid re-litigating later.

5. **Auto-Dev Agent Runner Spec**  
   - **File (proposed):** `docs/specs/auto_dev_agent_runner.md`  
   - **Status:** MISSING  
   - **Purpose:** Describe how your coding agents operate:  
     - Where they run (GitHub Actions vs VM).  
     - Available tools (edit files, run tests, git).  
     - Task lifecycle and safeguards.  

6. **Security & Privacy v0**  
   - **File (proposed):** `docs/security_and_privacy_v0.md`  
   - **Status:** MISSING  
   - **Purpose:** Baseline security posture, including:  
     - TLS/mTLS expectations.  
     - Telemetry redaction and sampling options.  
     - Self-host / tenant isolation notes.  

6. **Tamper-Evident Audit Log & Chain-of-Custody**
   - **File (proposed):** `docs/tamper_evident_audit.md`
   - **Status:** MISSING
   - **Purpose:**
     - Append-only event log + hash chaining for integrity (optional).
     - Separate “policy change log” vs “decision log” vs “telemetry log”.
     - Exportable attestations for incident reviews.

7. **RBAC / Delegation / Approvals (“Human-in-the-loop”)**
   - **File (proposed):** `docs/rbac_and_approvals.md`
   - **Status:** MISSING
   - **Purpose:**
     - Roles: platform admin vs security admin vs team owner.
     - Approval workflows for high-risk tools (“break-glass” + time-bound grants).

8. **Enterprise Integrations (Future-proofing)**
   - **File (proposed):** `docs/enterprise_integrations.md`
   - **Status:** MISSING
   - **Purpose:** SIEM export, ticketing (Jira), incident comms, and identity providers.

---

## Phase 5 – GTM, Revenue & Future

**Goal:** Move from “cool system” to “real business” with users, revenue, and potential funding.

### Deliverables

1. **ICP & Design Partner Plan**  
   - **File (proposed):** `docs/gtm/icp_and_design_partner_plan.md`  
   - **Status:** MISSING  
   - **Purpose:**  
     - Define ideal customer profiles (AI-forward teams with messy agents/skills).  
     - List target companies/contacts.  
     - Spell out value exchange for design partners.  

2. **Pricing & Packaging Hypotheses**  
   - **File (proposed):** `docs/gtm/pricing_hypotheses.md`  
   - **Status:** MISSING  
   - **Purpose:**  
     - Free tier concept (monitor up to X agents/skills / Y events).  
     - Paid tiers (more agents/events, advanced guardrails, SSO, retention).  
     - Early assumptions and open questions.

3. **Pitch Lite Deck v0**  
   - **File (proposed):** `deck/pitch_v0/` (multi-slide deck)  
   - **Status:** PARTIAL  
   - **Purpose:** 8–12 slide deck summarizing:  
     - Problem → Why now → Product → Demo → Market context → Roadmap.  
   - **Existing sources:**  
     - `agent_mesh_market_analysis.pdf` and prior product framing.

4. **Milestones & Decision Gates**  
   - **File (proposed):** `docs/milestones_and_decision_gates.md`  
   - **Status:** MISSING  
   - **Purpose:**  
     - Define concrete milestones (v0 demo, N design partners, first paid pilot).  
     - For each, specify what you do if hit vs missed by date (invest more or pause/kill).  

5. **Company Ops Basics (Later)**  
   - **File (proposed):** `docs/company/ops_basics.md`  
   - **Status:** MISSING  
   - **Purpose:** Capture eventual basics: incorporation, equity, IP ownership, and data processing posture once real customers are involved.

6. **Security + IR Pricing & Packaging**
   - **File (proposed):** `docs/pricing_security_ir.md`
   - **Status:** MISSING
   - **Purpose:**
     - Package around *governed tool calls*, *agents under policy*, and *incident exports* (not just “nodes in a graph”).
     - Define a strong free tier: limited governed tools + limited retention.

7. **GTM Narrative for Security Buyer**
   - **File (proposed):** `docs/gtm_security_buyer.md`
   - **Status:** MISSING
   - **Purpose:** Map pains and language for:
     - AI platform teams (operability)
     - Security/AppSec/GRC (governance + evidence)
     - Incident Response (forensics + postmortems)

---

## Status Summary by Category

- **HAVE (long-form)**  
  - `agent_mesh_observatory_v0_spec.pdf` – initial spec.  
  - `agent_mesh_market_analysis.pdf` – market & competitive analysis.

- **PARTIAL (concept exists but needs docs):**  
  - Vision One-Pager (`docs/vision.md`)  
  - Product Tenets (`docs/product_tenets.md`)  
  - Market Summary (`docs/market_summary.md`)  
  - Stack Decisions (`docs/stack_decisions.md`)  
  - Component Tech Specs (to be split out from v0 spec PDF)  
  - Pitch Lite Deck v0

- **MISSING (must be created):**  
  - v0 Demo Definition  
  - Personal Runway & Scenarios  
  - LLM Cost Model  
  - Agent Guardrails Doc  
  - Repo Structure Spec  
  - Telemetry & Event Schema v0  
  - v0 Feature Checklist & Roadmap  
  - Agent Task Schema & Backlog  
  - CI/CD Setup  
  - System Architecture Overview & Diagrams  
  - Node Types & Extensibility Doc (agents/skills/tools/etc.)  
  - ADRs  
  - Auto-Dev Agent Runner Spec  
  - Security & Privacy v0  
  - ICP & Design Partner Plan  
  - Pricing Hypotheses  
  - Milestones & Decision Gates  
  - Company Ops Basics

This `phase.md` should be treated as the authoritative index for what exists, what is partially captured, and what still needs to be built.
