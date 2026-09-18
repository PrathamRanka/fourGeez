import { describe, expect, it } from "vitest";
import { createSellerSessionStore } from "@/features/auth/server/session-store";

describe("seller session store", () => {
  it("keeps access credentials server-side and expires opaque sessions", () => {
    let now = Date.parse("2026-09-18T12:00:00Z");
    const sessions = createSellerSessionStore({ now: () => now });
    const session = sessions.create({
      accessToken: "server-only-access-token",
      expiresAt: "2026-09-18T13:00:00Z",
      principal: {
        subject: "local:owner",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: null,
        onboardingComplete: false,
      },
    });

    expect(session.sessionId).not.toContain("server-only-access-token");
    expect(sessions.read(session.sessionId)).toMatchObject({
      accessToken: "server-only-access-token",
      principal: { email: "owner@example.com" },
    });

    now = Date.parse("2026-09-18T13:00:00Z");
    expect(sessions.read(session.sessionId)).toBeNull();
  });

  it("persists seller ownership and onboarding progress for the signed-in owner", () => {
    const sessions = createSellerSessionStore();
    const session = sessions.create({
      accessToken: "server-only-access-token",
      expiresAt: "2099-09-18T13:00:00Z",
      principal: {
        subject: "local:owner",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: null,
        onboardingComplete: false,
      },
    });

    expect(
      sessions.updatePrincipal(session.sessionId, {
        sellerId: "sel_123",
        onboardingComplete: true,
      }),
    ).toBe(true);
    expect(sessions.read(session.sessionId)?.principal).toMatchObject({
      sellerId: "sel_123",
      onboardingComplete: true,
    });

    sessions.destroy(session.sessionId);
    expect(sessions.read(session.sessionId)).toBeNull();
  });
});
