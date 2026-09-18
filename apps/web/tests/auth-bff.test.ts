import { beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "@/app/api/auth/sign-in/route";
import { getIdentityAdapter } from "@/features/auth/server/identity";
import { cookies } from "next/headers";

const cookieValues = new Map<string, string>();
const setCookie = vi.fn((name: string, value: string) => {
  cookieValues.set(name, value);
});

vi.mock("next/headers", () => ({ cookies: vi.fn() }));
vi.mock("@/features/auth/server/identity", () => ({
  getIdentityAdapter: vi.fn(),
  getLocalIdentityAdapter: vi.fn(() => null),
}));

describe("seller authentication BFF", () => {
  beforeEach(() => {
    cookieValues.clear();
    cookieValues.set("agentpay_seller_csrf", "csrf-token");
    setCookie.mockClear();
    vi.mocked(cookies).mockResolvedValue({
      get: (name: string) => {
        const value = cookieValues.get(name);
        return value ? { name, value } : undefined;
      },
      set: setCookie,
      delete: (name: string) => cookieValues.delete(name),
    } as never);
    vi.mocked(getIdentityAdapter).mockReturnValue({
      signUp: vi.fn(),
      verifyRegistration: vi.fn(),
      beginRecovery: vi.fn(),
      completeRecovery: vi.fn(),
      revoke: vi.fn(),
      validate: vi.fn().mockResolvedValue({
        ok: false,
        code: "session_revoked",
        error: "Your session has ended. Sign in again.",
      }),
      signIn: vi.fn().mockResolvedValue({
        ok: true,
        value: {
          accessToken: "server-only-token",
          expiresAt: "2099-09-18T00:00:00Z",
          principal: {
            subject: "owner-subject",
            email: "owner@example.com",
            name: "Northstar Research",
            sellerId: null,
            onboardingComplete: false,
          },
        },
      }),
    });
  });

  it("rejects a cross-origin sign-in even when the CSRF token matches", async () => {
    const response = await POST(
      new Request("http://localhost:3000/api/auth/sign-in", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Origin: "https://attacker.example",
          "X-AgentPay-CSRF": "csrf-token",
        },
        body: JSON.stringify({
          email: "owner@example.com",
          password: "correct-horse-battery-staple",
          returnTo: "/dashboard",
        }),
      }),
    );

    expect(response.status).toBe(403);
    expect(await response.json()).toEqual({ error: "Invalid request origin." });
    expect(getIdentityAdapter().signIn).not.toHaveBeenCalled();
  });

  it("creates an opaque HttpOnly session cookie and returns onboarding for a new seller", async () => {
    const response = await POST(
      new Request("http://localhost:3000/api/auth/sign-in", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Origin: "http://localhost:3000",
          "X-AgentPay-CSRF": "csrf-token",
        },
        body: JSON.stringify({
          email: "owner@example.com",
          password: "correct-horse-battery-staple",
          returnTo: "https://attacker.example",
        }),
      }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(await response.json()).toEqual({
      redirectTo: "/dashboard/onboarding",
    });
    expect(setCookie).toHaveBeenCalledWith(
      "agentpay_seller_session",
      expect.not.stringContaining("server-only-token"),
      expect.objectContaining({
        httpOnly: true,
        sameSite: "strict",
      }),
    );
  });
});
