import type { components } from "@contracts/openapi";
import { z } from "zod";

export type UUID = string;

export const ReceiptKindSchema = z.enum(["decision", "invocation"]);
export type ReceiptKind = z.infer<typeof ReceiptKindSchema>;

export const DecisionReceiptSchema = z.object({
  kind: z.literal("decision"),
  id: z.string(),
  ts: z.string(),
  decision_id: z.string().optional().nullable(),
  decision: z.enum(["allow", "deny"]),
  policy_hash: z.string().optional().nullable(),
  request_id: z.string().optional().nullable(),
  hash: z.string(),
  prev_hash: z.string().optional().nullable(),
  trace_id: z.string().optional().nullable(),
  span_id: z.string().optional().nullable(),
});
export type DecisionReceipt = z.infer<typeof DecisionReceiptSchema>;

export const InvocationReceiptSchema = z.object({
  kind: z.literal("invocation"),
  id: z.string(),
  ts: z.string(),
  decision_id: z.string().optional().nullable(),
  request_id: z.string().optional().nullable(),
  tool_name: z.string(),
  method: z.string(),
  path: z.string(),
  outcome: z.enum(["success", "denied", "error"]),
  status_code: z.number().optional().nullable(),
  latency_ms: z.number().optional().nullable(),
  policy_hash: z.string().optional().nullable(),
  policy_version: z.number().optional().nullable(),
  hash: z.string(),
  prev_hash: z.string().optional().nullable(),
  trace_id: z.string().optional().nullable(),
  span_id: z.string().optional().nullable(),
});
export type InvocationReceipt = z.infer<typeof InvocationReceiptSchema>;

export const ReceiptSchema = z.discriminatedUnion("kind", [DecisionReceiptSchema, InvocationReceiptSchema]);
export type Receipt = z.infer<typeof ReceiptSchema>;

export const ReceiptListSchema = z.object({
  items: z.array(ReceiptSchema).optional(),
  next_before: z.string().optional(),
});

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
