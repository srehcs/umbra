# Phased Plan – Agent Identity Control Plane

This is the canonical phased plan for the **Identity + Policy control plane at the action boundary**:
enforce intent-bound capabilities for both employee “shadow AI” interactions and tool-enabled agents.

Status legend:
- **HAVE** – Exists as a reasonably complete asset.
- **PARTIAL** – Concept exists but needs cleanup.
- **MISSING** – Needs to be created.

Known assets already in this repo:
- `identity_v0_spec_control_plane.pdf` – V0 product/tech spec (**HAVE**).
- `identity_market_analysis.pdf` – market & competitive analysis (**HAVE**).
- `identity_comparative_analysis.pdf` – comparisons + positioning vs adjacent approaches (**HAVE**).

---

## Phase 0 – Decision + ICP Lock (1–2 weeks)

**Goal:** pick the *first* integration surface where we can enforce and produce receipts.

Deliverables:
- ICP + buyer memo (**MISSING**): security+platform co-buyer, initial wedge.
- “Day-1 policies” list (**MISSING**): 5–10 policies that matter (e.g., block public model uploads of repo code; require approval for prod DB queries).
- Target connector list (**MISSING**): start with 2–3 high-leverage tools.

Success criteria:
- We can describe the first demo in one paragraph and one diagram.

---

## Phase 1 – Action Boundary MVP (PEP + PDP + Receipts) (2–4 weeks)

**Goal:** block/allow a real tool action with a clear rationale + a signed receipt.

Build:
- Policy Decision Point (PDP): allow/deny/allow-with-constraints.
- Policy store: versioned policy-as-code workflow + tests.
- Enforcement point (PEP) for *one* path:
  - Agent tool-call PEP (MCP/HTTP) **or**
  - User traffic PEP (proxy/gateway) for a narrow set of endpoints.
- Receipt schema + signing; minimal “ledger” store.

Demo:
- Prompt injection attempt → blocked privileged tool call → receipt explains why.

---

## Phase 2 – Credential Brokering + Approvals (2–4 weeks)

**Goal:** shift from “static allowlists” to **intent-bound capabilities**.

Build:
- Credential broker for 1–2 tools (time-bound, least-privilege tokens).
- Approval flow for high-risk actions (break-glass, time-bound overrides).
- Audit UI: timeline + search.

---

## Phase 3 – Expand Connectors + Org Rollout (4–8 weeks)

**Goal:** become valuable across a real team.

Build:
- Add connectors (GitHub/Jira/Slack/SQL/cloud) based on ICP.
- Policy bundles / templates (starter packs).
- Multi-environment support (dev/stage/prod contexts).
- Better evidence bundles (export + redaction).

---

## Phase 4 – Scale + “Enterprise Grade” (later)

Build:
- Multi-tenant SaaS or hardened single-tenant deployments.
- More expressive policy model (optional OPA/Cedar compatibility).
- Advanced investigations (graph correlation with Mesh, if pursued).

