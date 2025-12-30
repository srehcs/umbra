export type UUID = string;

export type ReceiptKind = "decision" | "invocation";

export type DecisionReceipt = {
  kind: "decision";
  id: UUID;
  ts: string;
  decision_id: UUID;
  decision: "allow" | "deny";
  policy_hash?: string;
  request_id?: string;
  hash: string;
  prev_hash?: string;
  trace_id?: string;
  span_id?: string;
};

export type InvocationReceipt = {
  kind: "invocation";
  id: UUID;
  ts: string;
  decision_id: UUID;
  request_id?: string;
  tool_name: string;
  method: string;
  path: string;
  outcome: "success" | "denied" | "error";
  status_code: number;
  latency_ms: number;
  policy_hash?: string;
  policy_version?: number;
  hash: string;
  prev_hash?: string;
  trace_id?: string;
  span_id?: string;
};

export type Receipt = DecisionReceipt | InvocationReceipt;

export type Tool = {
  id: UUID;
  name: string;
  kind: string;
  config?: Record<string, unknown>;
  tenant_id?: UUID;
  created_at?: string;
  updated_at?: string;
};

export type PolicyRow = {
  id: UUID;
  name: string;
  version: number;
  active: boolean;
  policy_hash: string;
  policy: Record<string, unknown>;
  updated_at: string;
};

export type ListResponse<T> = {
  items: T[];
  next_before?: string;
};
