import type { ListResponse, PolicyRow, Receipt, Tool } from "./types";

const BASE_URL = "/api/controlplane";
const TENANT_KEY = "umbra.tenant_id";

function getTenant(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem(TENANT_KEY) || "";
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...(opts.headers ? (opts.headers as Record<string, string>) : {}),
  };
  const tenant = getTenant();
  if (tenant) headers["x-umbra-tenant-id"] = tenant;
  const res = await fetch(`${BASE_URL}${path}`, { ...opts, headers });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `HTTP ${res.status}`);
  }
  return (await res.json()) as T;
}

export const api = {
  // Tools
  listTools: (signal?: AbortSignal) =>
    request<{ items: Tool[] }>("/v1/tools", signal ? { signal } : {}),
  createTool: (body: { name: string; kind: string; config: unknown }) =>
    request<{ id: string }>("/v1/tools", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }),
  activateTool: (id: string) =>
    request<{ ok: true }>(`/v1/tools/${id}/activate`, { method: "POST" }),

  // Policies
  listPolicies: (signal?: AbortSignal) =>
    request<{ items: PolicyRow[] }>("/v1/policies", signal ? { signal } : {}),
  createPolicy: (body: { name: string; policy: unknown }) =>
    request<{ id: string }>("/v1/policies", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }),
  updatePolicy: (id: string, body: { policy: unknown }) =>
    request<{ id: string }>(`/v1/policies/${id}`, {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }),
  activatePolicy: (id: string) =>
    request<{ ok: true }>(`/v1/policies/${id}/activate`, { method: "POST" }),
  getActivePolicy: () =>
    request<{ id: string; name: string; version: number; policy_hash: string; updated_at: string }>(
      "/v1/policies/active",
    ),
  simulatePolicyServer: (body: {
    actor_roles?: string[];
    method: string;
    path: string;
    policy?: unknown;
  }) =>
    request<{ decision: string; reason: string; rule_index?: number; policy_hash?: string; policy_version?: number }>(
      "/v1/policies/simulate",
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      },
    ),

  // Receipts
  listReceipts: (
    params: { limit?: number; kind?: string; q?: string; before?: string } = {},
    signal?: AbortSignal,
  ) => {
    const sp = new URLSearchParams();
    sp.set("limit", String(params.limit ?? 100));
    if (params.kind && params.kind !== "all") sp.set("kind", params.kind);
    if (params.q) sp.set("q", params.q);
    if (params.before) sp.set("before", params.before);
    return request<ListResponse<Receipt>>(
      `/v1/receipts?${sp.toString()}`,
      signal ? { signal } : {},
    );
  },
  verifyReceipts: (params: { kind?: "decision" | "invocation" | "all"; limit?: number } = {}) => {
    const sp = new URLSearchParams();
    if (params.kind) sp.set("kind", params.kind);
    if (params.limit) sp.set("limit", String(params.limit));
    return request<{
      ok: boolean;
      checked: number;
      kind: string;
      failure?: { receipt_id: string; code: string };
    }>(`/v1/receipts/verify?${sp.toString()}`, { method: "POST" });
  },
};
