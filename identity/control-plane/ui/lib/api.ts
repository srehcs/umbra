import type { ListResponse, PolicyRow, Receipt, Tool } from "./types";

const BASE_URL =
  process.env.NEXT_PUBLIC_CONTROLPLANE_URL?.replace(/\/$/, "") || "http://localhost:8080";

function getTenant(): string {
  if (typeof window === "undefined") return "";
  return localStorage.getItem("umbra.tenant") || "";
}

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...(opts.headers ? (opts.headers as Record<string, string>) : {}),
    "x-umbra-tenant-id": getTenant(),
  };
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
    request<{ items: Tool[] }>("/v1/tools", { signal }),
  createTool: (body: { name: string; endpoint: string }) =>
    request<{ id: string }>("/v1/tools", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    }),
  activateTool: (id: string) =>
    request<{ ok: true }>(`/v1/tools/${id}/activate`, { method: "POST" }),

  // Policies
  listPolicies: (signal?: AbortSignal) =>
    request<{ items: PolicyRow[] }>("/v1/policies", { signal }),
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

  // Receipts
  listReceipts: (
    params: { limit?: number; kind?: string; q?: string; before?: string } = {},
    signal?: AbortSignal,
  ) => {
    const sp = new URLSearchParams();
    sp.set("limit", String(params.limit ?? 100));
    if (params.kind) sp.set("kind", params.kind);
    if (params.q) sp.set("q", params.q);
    if (params.before) sp.set("before", params.before);
    return request<ListResponse<Receipt>>(`/v1/receipts?${sp.toString()}`, { signal });
  },
};
