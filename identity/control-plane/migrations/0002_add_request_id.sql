-- Add request_id to receipts for correlation
ALTER TABLE receipts_decision
  ADD COLUMN IF NOT EXISTS request_id TEXT;

ALTER TABLE receipts_invocation
  ADD COLUMN IF NOT EXISTS request_id TEXT;
