-- Add search_text columns for receipt search and trigram indexes
CREATE EXTENSION IF NOT EXISTS pg_trgm;

ALTER TABLE receipts_decision
  ADD COLUMN IF NOT EXISTS search_text TEXT GENERATED ALWAYS AS (
    coalesce(decision_id::text, '') || ' ' ||
    coalesce(request_id, '') || ' ' ||
    coalesce(trace_id, '') || ' ' ||
    coalesce(policy_hash, '') || ' ' ||
    coalesce(decision, '') || ' ' ||
    coalesce(body_json->'tool'->>'name', '') || ' ' ||
    coalesce(body_json->'actor'->>'id', '')
  ) STORED;

ALTER TABLE receipts_invocation
  ADD COLUMN IF NOT EXISTS search_text TEXT GENERATED ALWAYS AS (
    coalesce(decision_id::text, '') || ' ' ||
    coalesce(request_id, '') || ' ' ||
    coalesce(trace_id, '') || ' ' ||
    coalesce(tool_name, '') || ' ' ||
    coalesce(method, '') || ' ' ||
    coalesce(path, '') || ' ' ||
    coalesce(outcome, '') || ' ' ||
    coalesce(body_json->>'policy_hash', '')
  ) STORED;

CREATE INDEX IF NOT EXISTS idx_receipts_decision_search_text
  ON receipts_decision USING GIN (search_text gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_receipts_invocation_search_text
  ON receipts_invocation USING GIN (search_text gin_trgm_ops);
