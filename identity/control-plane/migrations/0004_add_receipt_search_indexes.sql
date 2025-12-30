-- Add indexes to optimize receipt search and export filters
CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_trace_id
  ON receipts_decision(tenant_id, trace_id);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_trace_id
  ON receipts_invocation(tenant_id, trace_id);

CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_hash
  ON receipts_decision(tenant_id, hash);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_hash
  ON receipts_invocation(tenant_id, hash);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_tool
  ON receipts_invocation(tenant_id, tool_name);

CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_decision
  ON receipts_decision(tenant_id, decision);

CREATE INDEX IF NOT EXISTS idx_receipts_decision_tool_name
  ON receipts_decision(tenant_id, (body_json->'tool'->>'name'));
