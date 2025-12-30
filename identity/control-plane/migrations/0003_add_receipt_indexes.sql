-- Add indexes to speed up receipt export filters
CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_request_id
  ON receipts_decision(tenant_id, request_id);

CREATE INDEX IF NOT EXISTS idx_receipts_decision_tenant_decision_id
  ON receipts_decision(tenant_id, decision_id);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_request_id
  ON receipts_invocation(tenant_id, request_id);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_tenant_decision_id
  ON receipts_invocation(tenant_id, decision_id);
