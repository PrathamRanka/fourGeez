import { beforeEach, describe, expect, it, vi } from "vitest";
import { POST as requestRecovery } from "@/app/api/auth/recovery/request/route";
import { POST as signUp } from "@/app/api/auth/sign-up/route";
import { POST as verify } from "@/app/api/auth/verify/route";
import { getIdentityAdapter } from "@/features/auth/server/identity";
import { cookies } from "next/headers";

const cookieValues = new Map<string, string>();

vi.mock("next/headers", () => ({ cookies: vi.fn() }));
vi.mock("@/features/auth/server/identity", () => ({
  getIdentityAdapter: vi.fn(),
  getLocalIdentityAdapter: vi.fn(() => null),
}));

function request(path: string, body: Record<string, string>) {
  return new Request(`http://localhost:3000${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Origin: "http://localhost:3000",
      "X-AgentPay-CSRF": "csrf-token",
    },
    body: JSON.stringify(body),
  });
}

describe("auth BFF error mapping", () => {
  beforeEach(() => {
    vi.stubEnv("AGENTPAY_ENV", "local");
    vi.stubEnv("AGENTPAY_WEB_ORIGIN", "http://localhost:3000");
    cookieValues.clear();
    cookieValues.set("agentpay_seller_csrf", "csrf-token");
    cookieValues.set("agentpay_pending_verification", "challenge-id");
    vi.mocked(cookies).mockResolvedValue({
      get: (name: string) => {
        const value = cookieValues.get(name);
        return value ? { name, value } : undefined;
      },
      set: (name: string, value: string) => cookieValues.set(name, value),
      delete: (name: string) => cookieValues.delete(name),
    } as never);
  });

  it("returns 503 when registration or verification dependencies fail", async () => {
    vi.mocked(getIdentityAdapter).mockReturnValue({
      signUp: vi.fn().mockResolvedValue({
        ok: false,
        code: "dependency_unavailable",
        error: "Seller authentication is temporarily unavailable.",
      }),
      verifyRegistration: vi.fn().mockResolvedValue({
        ok: false,
        code: "dependency_unavailable",
        error: "Seller authentication is temporarily unavailable.",
      }),
      signIn: vi.fn(),
      beginRecovery: vi.fn(),
      completeRecovery: vi.fn(),
      validate: vi.fn(),
      revoke: vi.fn(),
    });

    expect(
      (
        await signUp(
          request("/api/auth/sign-up", {
            email: "owner@example.com",
            name: "Owner",
            password: "Correct-Horse-1!",
          }),
        )
      ).status,
    ).toBe(503);
    expect(
      (
        await verify(
          request("/api/auth/verify", {
            code: "123456",
          }),
        )
      ).status,
    ).toBe(503);
  });

  it("returns 400 for invalid recovery input instead of a dependency outage", async () => {
    vi.mocked(getIdentityAdapter).mockReturnValue({
      signUp: vi.fn(),
      verifyRegistration: vi.fn(),
      signIn: vi.fn(),
      beginRecovery: vi.fn().mockResolvedValue({
        ok: false,
        code: "invalid_request",
        error: "Enter a valid email address.",
      }),
      completeRecovery: vi.fn(),
      validate: vi.fn(),
      revoke: vi.fn(),
    });

    expect(
      (
        await requestRecovery(
          request("/api/auth/recovery/request", { email: "invalid" }),
        )
      ).status,
    ).toBe(400);
  });
});
