import { beforeEach, describe, expect, it, vi } from "vitest";
import { getIdentityAdapter } from "@/features/auth/server/identity";
import { sealSellerSession } from "@/features/auth/server/sealed-session";
import { refreshSellerSessionValue } from "@/features/auth/server/session";

vi.mock("@/features/auth/server/identity", () => ({
  getIdentityAdapter: vi.fn(),
  getLocalIdentityAdapter: vi.fn(() => null),
}));

const encryptionKey = Buffer.alloc(32, 11).toString("base64url");

describe("production seller session refresh", () => {
  beforeEach(() => {
    vi.stubEnv("AGENTPAY_ENV", "dev");
    vi.stubEnv("AGENTPAY_IDENTITY_MODE", "cognito");
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "https://agentpay.prathamranka.in");
    vi.stubEnv("AGENTPAY_SESSION_ENCRYPTION_KEY", encryptionKey);
  });

  it("rotates an expired access token while the absolute session is active", async () => {
    vi.mocked(getIdentityAdapter).mockReturnValue({
      signUp: vi.fn(),
      verifyRegistration: vi.fn(),
      signIn: vi.fn(),
      beginRecovery: vi.fn(),
      completeRecovery: vi.fn(),
      validate: vi.fn(),
      revoke: vi.fn(),
      refresh: vi.fn().mockResolvedValue({
        ok: true,
        value: {
          accessToken: "refreshed-access-token",
          refreshToken: "refresh-token",
          expiresAt: new Date(Date.now() + 60 * 60 * 1_000).toISOString(),
          sessionExpiresAt: new Date(Date.now() + 6 * 60 * 60 * 1_000).toISOString(),
          principal: {
            subject: "seller-subject",
            email: "owner@example.com",
            name: "Owner",
            sellerId: "sel_123",
            onboardingComplete: true,
          },
        },
      }),
    });
    const sealed = sealSellerSession(
      {
        accessToken: "expired-access-token",
        refreshToken: "refresh-token",
        expiresAt: new Date(Date.now() - 60_000).toISOString(),
        sessionExpiresAt: new Date(Date.now() + 6 * 60 * 60 * 1_000).toISOString(),
        principal: {
          subject: "seller-subject",
          email: "owner@example.com",
          name: "Owner",
          sellerId: "sel_123",
          onboardingComplete: true,
        },
      },
      encryptionKey,
    );

    const result = await refreshSellerSessionValue(sealed);

    expect(result).toMatchObject({ status: "refreshed" });
    expect(getIdentityAdapter().refresh).toHaveBeenCalledOnce();
    if (result.status === "refreshed") {
      expect(result.cookieValue).not.toContain("refreshed-access-token");
    }
  });

  it("expires the seller session at its absolute boundary", async () => {
    const sealed = sealSellerSession(
      {
        accessToken: "expired-access-token",
        refreshToken: "refresh-token",
        expiresAt: new Date(Date.now() - 60_000).toISOString(),
        sessionExpiresAt: new Date(Date.now() - 1).toISOString(),
        principal: {
          subject: "seller-subject",
          email: "owner@example.com",
          name: "Owner",
          sellerId: "sel_123",
          onboardingComplete: true,
        },
      },
      encryptionKey,
    );

    await expect(refreshSellerSessionValue(sealed)).resolves.toEqual({
      status: "expired",
    });
  });
});
