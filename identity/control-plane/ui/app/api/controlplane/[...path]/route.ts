const CONTROLPLANE_URL =
  process.env.CONTROLPLANE_API_URL?.replace(/\/$/, "") || "http://localhost:8080";

async function proxy(request: Request, params: { path?: string[] }) {
  const targetPath = params.path?.join("/") || "";
  const incomingUrl = new URL(request.url);
  const targetUrl = new URL(`${CONTROLPLANE_URL}/${targetPath}`);
  targetUrl.search = incomingUrl.search;

  const headers = new Headers(request.headers);
  headers.delete("host");

  let body: ArrayBuffer | null = null;
  if (request.method !== "GET" && request.method !== "HEAD") {
    body = await request.arrayBuffer();
  }

  const resp = await fetch(targetUrl, {
    method: request.method,
    headers,
    body,
  });

  if (!resp.ok) {
    console.error("controlplane proxy error", {
      status: resp.status,
      path: `/${targetPath}`,
    });
  }

  return new Response(resp.body, {
    status: resp.status,
    headers: resp.headers,
  });
}

export async function GET(request: Request, ctx: { params: { path?: string[] } }) {
  return proxy(request, ctx.params);
}

export async function POST(request: Request, ctx: { params: { path?: string[] } }) {
  return proxy(request, ctx.params);
}

export async function PUT(request: Request, ctx: { params: { path?: string[] } }) {
  return proxy(request, ctx.params);
}

export async function PATCH(request: Request, ctx: { params: { path?: string[] } }) {
  return proxy(request, ctx.params);
}

export async function DELETE(request: Request, ctx: { params: { path?: string[] } }) {
  return proxy(request, ctx.params);
}
