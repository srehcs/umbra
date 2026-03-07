import { NextResponse } from "next/server";
import { headers } from "next/headers";

const authEnabled = process.env.AUTH_ENABLED === "true";

export async function GET() {
  if (!authEnabled) {
    return NextResponse.json({ error: "auth disabled" }, { status: 404 });
  }
  const h = headers();
  const user = h.get("x-umbra-claim-sub") ?? h.get("x-umbra-user");
  const roles = (h.get("x-umbra-claim-roles") ?? h.get("x-umbra-roles") ?? "")
    .split(",")
    .map((role) => role.trim())
    .filter(Boolean);
  const tenantId = h.get("x-umbra-claim-tenant-id") ?? h.get("x-umbra-tenant-id") ?? undefined;
  if (!user || !tenantId) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  return NextResponse.json({
    user: { id: user },
    roles,
    tenant_id: tenantId ?? undefined,
  });
}
