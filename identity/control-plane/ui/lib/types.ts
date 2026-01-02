import type { components } from "@contracts/openapi";

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

export type Tool = components["schemas"]["Tool"];

export type CreateToolRequest = components["schemas"]["CreateToolRequest"];

export type PolicyRow = components["schemas"]["Policy"];

export type CreatePolicyRequest = components["schemas"]["CreatePolicyRequest"];

export type ActivePolicyResponse = components["schemas"]["ActivePolicyResponse"];

export type SimulateResponse = components["schemas"]["SimulateResponse"];

export type SimulateRequest = components["schemas"]["SimulateRequest"];

export type ListResponse<T> = {
  items: T[];
  next_before?: string;
};
