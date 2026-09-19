import { createHash } from "node:crypto";
import { cookies } from "next/headers";
import type { IdentityAuthentication } from "@/features/auth/model";
import {
  getIdentityAdapter,
  getLocalIdentityAdapter,
} from "@/features/auth/server/identity";
import {
  createSellerSessionStore,
  type SellerSession,
} from "@/features/auth/server/session-store";
import {
  openSellerSession,
  sealSellerSession,
} from "@/features/auth/server/sealed-session";

const sessionCookieMaxAgeSeconds = 8 * 60 * 60;

declare global {
  var agentPaySellerSessions:
    ReturnType<typeof createSellerSessionStore> | undefined;
}

export function sellerSessionCookieName(): string {
  return secureCookiesEnabled()
    ? "__Host-agentpay_seller_session"
    : "agentpay_seller_session";
}

function secureCookiesEnabled(): boolean {
  return (
    process.env.AGENTPAY_ENV !== "local" &&
    (process.env.AGENTPAY_WEB_ORIGIN ?? "").startsWith("https://")
  );
}

function productionSessionsEnabled(): boolean {
  return secureCookiesEnabled();
}

function sessionEncryptionKey(): string {
  const encryptionKey =
    process.env.AGENTPAY_SESSION_ENCRYPTION_KEY?.trim() ?? "";
  if (!encryptionKey) {
    throw new Error(
      "AGENTPAY_SESSION_ENCRYPTION_KEY is required for production seller sessions.",
    );
  }
  return encryptionKey;
}

function sessionIdFromCookie(cookieValue: string): string {
  return createHash("sha256").update(cookieValue).digest("base64url");
}

function sessionCookieOptions(expiresAt: string) {
  return {
    httpOnly: true,
    maxAge: Math.min(
      sessionCookieMaxAgeSeconds,
      Math.max(0, Math.floor((Date.parse(expiresAt) - Date.now()) / 1_000)),
    ),
    path: "/" as const,
    sameSite: "strict" as const,
    secure: secureCookiesEnabled(),
  };
}

function sessionStore() {
  globalThis.agentPaySellerSessions ??= createSellerSessionStore();
  return globalThis.agentPaySellerSessions;
}

export async function establishSellerSession(
  authentication: IdentityAuthentication,
): Promise<SellerSession> {
  if (productionSessionsEnabled()) {
    const cookieValue = sealSellerSession(
      authentication,
      sessionEncryptionKey(),
    );
    const cookieStore = await cookies();
    cookieStore.set(
      sellerSessionCookieName(),
      cookieValue,
      sessionCookieOptions(
        authentication.sessionExpiresAt ?? authentication.expiresAt,
      ),
    );
    return {
      ...authentication,
      sessionId: sessionIdFromCookie(cookieValue),
    };
  }
  const session = sessionStore().create(authentication);
  const cookieStore = await cookies();
  cookieStore.set(
    sellerSessionCookieName(),
    session.sessionId,
    sessionCookieOptions(session.expiresAt),
  );
  return session;
}

export async function getSellerSession(): Promise<SellerSession | null> {
  const cookieStore = await cookies();
  const cookieValue = cookieStore.get(sellerSessionCookieName())?.value;
  if (!cookieValue) return null;
  const session = productionSessionsEnabled()
    ? openSellerSession(cookieValue, sessionEncryptionKey())
    : sessionStore().read(cookieValue);
  if (!session) return null;
  const validation = await getIdentityAdapter().validate(session);
  if (!validation.ok) {
    if (!productionSessionsEnabled()) sessionStore().destroy(cookieValue);
    return null;
  }
  return {
    ...validation.value,
    sessionId: productionSessionsEnabled()
      ? sessionIdFromCookie(cookieValue)
      : cookieValue,
    principal: { ...validation.value.principal },
  };
}

async function revokeAPISession(accessToken: string): Promise<boolean> {
  if (!productionSessionsEnabled()) return true;
  const apiOrigin = process.env.AGENTPAY_API_ORIGIN?.trim().replace(/\/$/, "");
  if (!apiOrigin) return false;
  try {
    const response = await fetch(`${apiOrigin}/v1/me/session`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${accessToken}` },
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
    });
    return response.status === 204 || response.status === 401;
  } catch {
    return false;
  }
}

export async function destroySellerSession(): Promise<boolean> {
  const cookieStore = await cookies();
  const cookieName = sellerSessionCookieName();
  const cookieValue = cookieStore.get(cookieName)?.value;
  let revocationConfirmed = true;
  if (cookieValue) {
    const session = productionSessionsEnabled()
      ? openSellerSession(cookieValue, sessionEncryptionKey())
      : sessionStore().read(cookieValue);
    if (session) {
      revocationConfirmed = await revokeAPISession(session.accessToken);
      await getIdentityAdapter().revoke(session);
    }
    if (!productionSessionsEnabled()) sessionStore().destroy(cookieValue);
  }
  cookieStore.delete(cookieName);
  return revocationConfirmed;
}

export async function updateCurrentSellerPrincipal(input: {
  onboardingComplete: boolean;
  sellerId: string | null;
  storefront?: SellerSession["principal"]["storefront"];
}): Promise<boolean> {
  const cookieStore = await cookies();
  const cookieName = sellerSessionCookieName();
  const cookieValue = cookieStore.get(cookieName)?.value;
  if (!cookieValue) return false;
  if (productionSessionsEnabled()) {
    const authentication = openSellerSession(
      cookieValue,
      sessionEncryptionKey(),
    );
    if (!authentication) return false;
    const updated = {
      ...authentication,
      principal: { ...authentication.principal, ...input },
    };
    cookieStore.set(
      cookieName,
      sealSellerSession(updated, sessionEncryptionKey()),
      sessionCookieOptions(updated.sessionExpiresAt ?? updated.expiresAt),
    );
    return true;
  }
  const session = sessionStore().read(cookieValue);
  if (!session) return false;
  const updated = sessionStore().updatePrincipal(cookieValue, input);
  getLocalIdentityAdapter()?.updatePrincipal(session.principal.subject, input);
  return updated;
}

export type SellerSessionRefresh =
  | { status: "current" }
  | { status: "expired" }
  | { status: "refreshed"; cookieValue: string; expiresAt: string };

export async function refreshSellerSessionValue(
  cookieValue: string,
): Promise<SellerSessionRefresh> {
  if (!productionSessionsEnabled()) return { status: "current" };
  const authentication = openSellerSession(
    cookieValue,
    sessionEncryptionKey(),
  );
  if (!authentication) return { status: "expired" };
  if (Date.parse(authentication.expiresAt) > Date.now() + 30_000) {
    return { status: "current" };
  }
  const refresh = getIdentityAdapter().refresh;
  if (!refresh) return { status: "expired" };
  const refreshed = await refresh(authentication);
  if (!refreshed.ok) return { status: "expired" };
  return {
    status: "refreshed",
    cookieValue: sealSellerSession(refreshed.value, sessionEncryptionKey()),
    expiresAt: refreshed.value.sessionExpiresAt ?? refreshed.value.expiresAt,
  };
}
