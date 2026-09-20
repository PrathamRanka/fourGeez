const requestHeaderAllowlist = [
  "accept",
  "accept-language",
  "authorization",
  "content-type",
  "idempotency-key",
  "payment-signature",
  "x-payment",
  "x-agentpay-agent-key",
  "x-agentpay-csrf",
  "x-agentpay-intent-id",
  "x-agentpay-project-key",
  "x-agentpay-request-id",
] as const;

const responseHeaderAllowlist = [
  "allow",
  "cache-control",
  "content-disposition",
  "content-language",
  "content-type",
  "etag",
  "last-modified",
  "payment-required",
  "payment-response",
  "retry-after",
  "vary",
  "www-authenticate",
  "x-agentpay-request-id",
  "x-agentpay-retry-safe",
  "x-agentpay-transaction-id",
] as const;

type BackendRouteContext = {
  params: Promise<{ path: string[] }>;
};

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

function copyHeaders(source: Headers, names: readonly string[]): Headers {
  const headers = new Headers();
  for (const name of names) {
    const value = source.get(name);
    if (value !== null) headers.set(name, value);
  }
  return headers;
}

function unavailable(message: string): Response {
  return Response.json(
    { error: { code: "backend_unavailable", message } },
    { status: 503, headers: { "Cache-Control": "no-store" } },
  );
}

function upstreamOrigin(requestURL: URL): URL | null {
  const configured = process.env.AGENTPAY_API_ORIGIN?.trim();
  if (!configured) return null;

  let origin: URL;
  try {
    origin = new URL(configured.endsWith("/") ? configured : `${configured}/`);
  } catch {
    return null;
  }

  if (origin.username || origin.password || origin.search || origin.hash) {
    return null;
  }

  const normalizedPath = origin.pathname.replace(/\/+$/, "");
  if (
    origin.origin === requestURL.origin &&
    (normalizedPath === "/api/backend" ||
      normalizedPath.startsWith("/api/backend/"))
  ) {
    return null;
  }

  return origin;
}

async function proxyBackend(
  request: Request,
  context: BackendRouteContext,
): Promise<Response> {
  const requestURL = new URL(request.url);
  const origin = upstreamOrigin(requestURL);
  if (!origin) {
    return unavailable("The AgentPay backend is not configured.");
  }

  const { path } = await context.params;
  const encodedPath = path.map((segment) => encodeURIComponent(segment)).join("/");
  const upstreamURL = new URL(encodedPath, origin);
  upstreamURL.search = requestURL.search;

  const method = request.method.toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await request.arrayBuffer();

  try {
    const upstream = await fetch(upstreamURL.toString(), {
      method,
      headers: copyHeaders(request.headers, requestHeaderAllowlist),
      body,
      cache: "no-store",
      redirect: "manual",
    });

    return new Response(upstream.body, {
      status: upstream.status,
      statusText: upstream.statusText,
      headers: copyHeaders(upstream.headers, responseHeaderAllowlist),
    });
  } catch {
    return Response.json(
      {
        error: {
          code: "backend_unreachable",
          message: "AgentPay could not reach the backend.",
        },
      },
      { status: 502, headers: { "Cache-Control": "no-store" } },
    );
  }
}

export const GET = proxyBackend;
export const HEAD = proxyBackend;
export const POST = proxyBackend;
export const PUT = proxyBackend;
export const PATCH = proxyBackend;
export const DELETE = proxyBackend;
export const OPTIONS = proxyBackend;
