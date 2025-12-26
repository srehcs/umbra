-- Umbra V0-C schema (tenant-aware)
-- Enable pgcrypto for UUID generation
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tenants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tools (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('http', 'mcp', 'cli')),
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tools_tenant_created ON tools(tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS policies (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  version INT NOT NULL DEFAULT 1,
  active BOOLEAN NOT NULL DEFAULT FALSE,
  policy_json JSONB NOT NULL,
  policy_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, name, version)
);

CREATE INDEX IF NOT EXISTS idx_policies_tenant_active ON policies(tenant_id, active, updated_at DESC);

-- Receipts: signing-ready columns included now (optional)
CREATE TABLE IF NOT EXISTS receipts_decision (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  decision_id UUID NOT NULL,
  policy_hash TEXT NOT NULL,
  decision TEXT NOT NULL CHECK (decision IN ('allow', 'deny')),
  body_json JSONB NOT NULL,
  prev_hash TEXT,
  hash TEXT NOT NULL,
  trace_id TEXT,
  span_id TEXT,
  signature_alg TEXT,
  signature_kid TEXT,
  signature TEXT,
  signed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_ts ON receipts_decision(tenant_id, ts DESC);

CREATE TABLE IF NOT EXISTS receipts_invocation (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  decision_id UUID,
  tool_name TEXT NOT NULL,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  outcome TEXT NOT NULL CHECK (outcome IN ('success', 'error', 'denied')),
  status_code INT,
  latency_ms INT NOT NULL,
  body_json JSONB NOT NULL,
  prev_hash TEXT,
  hash TEXT NOT NULL,
  trace_id TEXT,
  span_id TEXT,
  signature_alg TEXT,
  signature_kid TEXT,
  signature TEXT,
  signed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_ts ON receipts_invocation(tenant_id, ts DESC);
