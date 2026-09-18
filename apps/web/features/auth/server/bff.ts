import { randomBytes } from "node:crypto";
import { cookies } from "next/headers";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { validateCsrfRequest } from "@/features/auth/server/csrf";

const maximumAuthBodyBytes = 16_384;

export const pendingVerificationCookie = "agentpay_pending_verification";
export const pendingRecoveryCookie = "agentpay_pending_recovery";

export function sellerCsrfCookieName(): string {
  return secureCookiesEnabled()
    ? "__Host-agentpay_seller_csrf"
    : "agentpay_seller_csrf";
}

function secureCookiesEnabled(): boolean {
  return (
    process.env.AGENTPAY_ENV !== "local" && process.env.NODE_ENV !== "test"
  );
}

function cookieOptions(httpOnly: boolean) {
  return {
    httpOnly,
    path: "/" as const,
    sameSite: "strict" as const,
    secure: secureCookiesEnabled(),
  };
}

export async function issueCsrfToken(): Promise<string> {
  const cookieStore = await cookies();
  const cookieName = sellerCsrfCookieName();
  const existing = cookieStore.get(cookieName)?.value;
  if (existing) return existing;
  const token = randomBytes(32).toString("base64url");
  cookieStore.set(cookieName, token, cookieOptions(false));
  return token;
}

export async function requireCsrf(request: Request) {
  const cookieStore = await cookies();
  return validateCsrfRequest({
    allowedOrigin: (
      process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000"
    ).replace(/\/$/, ""),
    cookieToken: cookieStore.get(sellerCsrfCookieName())?.value,
    headerToken: request.headers.get("X-AgentPay-CSRF"),
    origin: request.headers.get("Origin"),
  });
}

export async function readAuthBody(
  request: Request,
  allowedFields: readonly string[],
): Promise<Record<string, unknown> | null> {
  if (!request.headers.get("Content-Type")?.startsWith("application/json")) {
    return null;
  }
  const bodyText = await request.text();
  if (
    !bodyText ||
    new TextEncoder().encode(bodyText).byteLength > maximumAuthBodyBytes
  ) {
    return null;
  }
  try {
    const body: unknown = JSON.parse(bodyText);
    if (!body || typeof body !== "object" || Array.isArray(body)) return null;
    if (Object.keys(body).some((field) => !allowedFields.includes(field)))
      return null;
    return body as Record<string, unknown>;
  } catch {
    return null;
  }
}

export async function setPendingChallenge(
  cookieName: string,
  challengeId: string,
): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.set(cookieName, challengeId, {
    ...cookieOptions(true),
    maxAge: 10 * 60,
  });
}

export async function readPendingChallenge(cookieName: string) {
  return (await cookies()).get(cookieName)?.value ?? null;
}

export async function clearPendingChallenge(cookieName: string) {
  (await cookies()).delete(cookieName);
}

export function authJson(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("Cache-Control", "no-store");
  return Response.json(body, { ...init, headers });
}

export function authError(error: string, status: number): Response {
  return authJson({ error }, { status });
}

export function authRedirectPath(path: string): string {
  return safeRelativeReturnPath(path);
}
