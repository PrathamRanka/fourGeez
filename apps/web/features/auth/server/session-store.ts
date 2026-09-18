import { randomBytes } from "node:crypto";
import type {
  IdentityAuthentication,
  SellerPrincipal,
} from "@/features/auth/model";

export type SellerSession = IdentityAuthentication & {
  sessionId: string;
};

type SellerSessionStoreOptions = {
  now?: () => number;
};

function newSessionId(): string {
  return randomBytes(32).toString("base64url");
}

// createSellerSessionStore owns opaque browser-session state for one web process.
export function createSellerSessionStore(
  options: SellerSessionStoreOptions = {},
) {
  const now = options.now ?? Date.now;
  const sessions = new Map<string, SellerSession>();

  return {
    create(authentication: IdentityAuthentication): SellerSession {
      const session = {
        ...authentication,
        principal: { ...authentication.principal },
        sessionId: newSessionId(),
      };
      sessions.set(session.sessionId, session);
      return session;
    },

    read(sessionId: string): SellerSession | null {
      const session = sessions.get(sessionId);
      if (!session || Date.parse(session.expiresAt) <= now()) {
        sessions.delete(sessionId);
        return null;
      }
      return {
        ...session,
        principal: { ...session.principal },
      };
    },

    updatePrincipal(
      sessionId: string,
      update: Pick<SellerPrincipal, "onboardingComplete" | "sellerId"> &
        Partial<Pick<SellerPrincipal, "storefront">>,
    ): boolean {
      const session = sessions.get(sessionId);
      if (!session || Date.parse(session.expiresAt) <= now()) {
        sessions.delete(sessionId);
        return false;
      }
      session.principal = { ...session.principal, ...update };
      return true;
    },

    destroy(sessionId: string): void {
      sessions.delete(sessionId);
    },
  };
}
