ALTER TABLE receipts_decision
ADD COLUMN IF NOT EXISTS body_canonical bytea;

ALTER TABLE receipts_invocation
ADD COLUMN IF NOT EXISTS body_canonical bytea;
