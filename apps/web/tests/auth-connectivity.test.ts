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

describe("production seller auth connectivity", () => {
  beforeEach(() => {
    vi.stubEnv("AGENTPAY_ENV", "dev");
    vi.stubEnv("AGENTPAY_IDENTITY_MODE", "cognito");
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "https://agentpay.prathamranka.in");
    vi.stubEnv("AGENTPAY_API_ORIGIN", "https://api.example.test");
    vi.stubEnv(
      "AGENTPAY_SESSION_ENCRYPTION_KEY",
      Buffer.alloc(32, 9).toString("base64url"),
    );
    cookieValues.clear();
    cookieValues.set("__Host-agentpay_seller_csrf", "csrf-token");
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
      validate: vi.fn(),
      signIn: vi.fn().mockResolvedValue({
        ok: true,
        value: {
          accessToken: "server-only-token",
          expiresAt: "2099-09-19T00:00:00Z",
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

  it("hydrates claim-derived seller ownership before creating a Vercel session", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: string | URL | Request) => {
        const url = String(input);
        if (url.endsWith("/v1/me/seller")) {
          return Response.json({
            sellerId: "sel_123",
            name: "Northstar Research",
            slug: "northstar",
            upstreamBaseUrl: "https://seller.example",
            status: "active",
            createdAt: "2026-09-19T09:00:00Z",
            updatedAt: "2026-09-19T09:00:00Z",
            version: 3,
          });
        }
        if (url.endsWith("/v1/me/onboarding")) {
          return Response.json({
            sellerId: "sel_123",
            complete: true,
            currentStep: "complete",
            steps: [],
            publication: {},
            version: 3,
          });
        }
        throw new Error(`Unexpected request: ${url}`);
      }),
    );

    const response = await POST(
      new Request("https://agentpay.prathamranka.in/api/auth/sign-in", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Origin: "https://agentpay.prathamranka.in",
          "X-AgentPay-CSRF": "csrf-token",
        },
        body: JSON.stringify({
          email: "owner@example.com",
          password: "correct-horse-battery-staple",
          returnTo: "/dashboard/products",
        }),
      }),
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      redirectTo: "/dashboard/products",
    });
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(setCookie).toHaveBeenCalledWith(
      "__Host-agentpay_seller_session",
      expect.not.stringContaining("server-only-token"),
      expect.objectContaining({
        httpOnly: true,
        sameSite: "strict",
        secure: true,
      }),
    );
  });
});
