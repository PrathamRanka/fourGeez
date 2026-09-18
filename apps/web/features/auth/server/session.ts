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
    process.env.AGENTPAY_ENV !== "local" && process.env.NODE_ENV !== "test"
  );
}

function sessionStore() {
  globalThis.agentPaySellerSessions ??= createSellerSessionStore();
  return globalThis.agentPaySellerSessions;
}

export async function establishSellerSession(
  authentication: IdentityAuthentication,
): Promise<SellerSession> {
  const session = sessionStore().create(authentication);
  const cookieStore = await cookies();
  cookieStore.set(sellerSessionCookieName(), session.sessionId, {
    httpOnly: true,
    maxAge: Math.min(
      sessionCookieMaxAgeSeconds,
      Math.max(
        0,
        Math.floor((Date.parse(session.expiresAt) - Date.now()) / 1_000),
      ),
    ),
    path: "/",
    sameSite: "strict",
    secure: secureCookiesEnabled(),
  });
  return session;
}

export async function getSellerSession(): Promise<SellerSession | null> {
  const cookieStore = await cookies();
  const sessionId = cookieStore.get(sellerSessionCookieName())?.value;
  if (!sessionId) return null;
  const session = sessionStore().read(sessionId);
  if (!session) return null;
  const validation = await getIdentityAdapter().validate(session.accessToken);
  if (!validation.ok) {
    sessionStore().destroy(sessionId);
    return null;
  }
  return {
    ...session,
    principal: { ...validation.value.principal },
  };
}

export async function destroySellerSession(): Promise<void> {
  const cookieStore = await cookies();
  const cookieName = sellerSessionCookieName();
  const sessionId = cookieStore.get(cookieName)?.value;
  if (sessionId) {
    const session = sessionStore().read(sessionId);
    if (session) await getIdentityAdapter().revoke(session.accessToken);
    sessionStore().destroy(sessionId);
  }
  cookieStore.delete(cookieName);
}

export async function updateCurrentSellerPrincipal(input: {
  onboardingComplete: boolean;
  sellerId: string | null;
  storefront?: SellerSession["principal"]["storefront"];
}): Promise<boolean> {
  const cookieStore = await cookies();
  const sessionId = cookieStore.get(sellerSessionCookieName())?.value;
  if (!sessionId) return false;
  const session = sessionStore().read(sessionId);
  if (!session) return false;
  const updated = sessionStore().updatePrincipal(sessionId, input);
  getLocalIdentityAdapter()?.updatePrincipal(session.principal.subject, input);
  return updated;
}
