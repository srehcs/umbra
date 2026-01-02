import { createApiClient } from "@contracts/client";
import type { components } from "@contracts/openapi";
import type {
  ActivePolicyResponse,
  CreatePolicyRequest,
  CreateToolRequest,
  ListResponse,
  PolicyRow,
  Receipt,
  SimulateRequest,
  SimulateResponse,
  Tool,
} from "./types";

const BASE_URL = "/api/controlplane";
const TENANT_KEY = "umbra.tenant_id";

type ToolList = components["schemas"]["ToolList"];
type PolicyList = components["schemas"]["PolicyList"];
type ReceiptList = components["schemas"]["ReceiptList"];
type ReceiptVerifyResponse = components["schemas"]["ReceiptVerifyResponse"];

function getTenant(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(TENANT_KEY) || "";
}

function getClient() {
  const headers: Record<string, string> = {};
  const tenant = getTenant();
  if (tenant) headers["x-umbra-tenant-id"] = tenant;
  return createApiClient({ baseUrl: BASE_URL, headers });
}

async function unwrap<T>(result: { data?: T; response?: Response }): Promise<T> {
  if (result.response?.ok && result.data !== undefined) {
    return result.data;
  }
  const text = await result.response?.text().catch(() => "");
  throw new Error(text || `HTTP ${result.response?.status ?? 500}`);
}

export const api = {
  // Tools
  listTools: async (signal?: AbortSignal): Promise<{ items: Tool[] }> => {
    const result = await getClient().GET("/v1/tools", signal ? { signal } : {});
    const data = await unwrap<ToolList>(result);
    return { items: (data.items ?? []) as Tool[] };
  },
  createTool: async (body: CreateToolRequest): Promise<{ id: string }> => {
    const result = await getClient().POST("/v1/tools", { body });
    const data = await unwrap<Tool>(result);
    return { id: data.id };
  },

  // Policies
  listPolicies: async (signal?: AbortSignal): Promise<{ items: PolicyRow[] }> => {
    const result = await getClient().GET("/v1/policies", signal ? { signal } : {});
    const data = await unwrap<PolicyList>(result);
    return { items: (data.items ?? []) as PolicyRow[] };
  },
  createPolicy: async (body: CreatePolicyRequest): Promise<{ id: string }> => {
    const result = await getClient().POST("/v1/policies", { body });
    const data = await unwrap<PolicyRow>(result);
    return { id: data.id };
  },
  updatePolicy: async (id: string, body: { policy: Record<string, unknown> }): Promise<{ id: string }> => {
    const res = await fetch(`${BASE_URL}/v1/policies/${id}`, {
      method: "PUT",
      headers: { "content-type": "application/json", "x-umbra-tenant-id": getTenant() },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(text || `HTTP ${res.status}`);
    }
    const data = (await res.json()) as PolicyRow;
    return { id: data.id };
  },
  activatePolicy: async (id: string): Promise<{ ok?: boolean }> => {
    const result = await getClient().POST("/v1/policies/activate", {
      body: { policy_id: id },
    });
    return unwrap(result);
  },
  getActivePolicy: async (): Promise<ActivePolicyResponse> => {
    const result = await getClient().GET("/v1/policies/active");
    return unwrap<ActivePolicyResponse>(result);
  },
  simulatePolicyServer: async (body: SimulateRequest): Promise<SimulateResponse> => {
    const result = await getClient().POST("/v1/policies/simulate", { body });
    return unwrap<SimulateResponse>(result);
  },

  // Receipts
  listReceipts: async (
    params: { limit?: number; kind?: "decision" | "invocation"; q?: string; before?: string } = {},
    signal?: AbortSignal,
  ): Promise<ListResponse<Receipt>> => {
    const result = await getClient().GET("/v1/receipts", {
      params: { query: params },
      ...(signal ? { signal } : {}),
    });
    const data = await unwrap<ReceiptList>(result);
    const next = data.next_before;
    return {
      items: (data.items ?? []) as Receipt[],
      ...(next ? { next_before: next } : {}),
    };
  },
  verifyReceipts: async (params: { kind?: "decision" | "invocation" | "all"; limit?: number } = {}): Promise<ReceiptVerifyResponse> => {
    const result = await getClient().POST("/v1/receipts/verify", {
      params: { query: params },
    });
    return unwrap<ReceiptVerifyResponse>(result);
  },
};
