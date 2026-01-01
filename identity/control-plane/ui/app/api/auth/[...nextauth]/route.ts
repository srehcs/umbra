import { NextResponse } from "next/server";

const authEnabled = process.env.AUTH_ENABLED === "true";

function disabledResponse() {
  return NextResponse.json({ error: "auth disabled" }, { status: 404 });
}

function notConfiguredResponse() {
  return NextResponse.json({ error: "auth enabled but not configured" }, { status: 501 });
}

export async function GET() {
  if (!authEnabled) return disabledResponse();
  return notConfiguredResponse();
}

export async function POST() {
  if (!authEnabled) return disabledResponse();
  return notConfiguredResponse();
}
