import { describe, expect, it } from "vitest";
import {
  postAuthenticationPath,
  safeRelativeReturnPath,
} from "@/features/auth/policy";
import { validateCsrfRequest } from "@/features/auth/server/csrf";

describe("seller authentication policy", () => {
  it.each([
    ["/dashboard/products?status=draft", "/dashboard/products?status=draft"],
    ["/dashboard", "/dashboard"],
    ["https://attacker.example", "/dashboard"],
    ["//attacker.example", "/dashboard"],
    ["/\\attacker.example", "/dashboard"],
    ["/%2f%2fattacker.example", "/dashboard"],
    ["/dashboard#token", "/dashboard"],
  ])("normalizes return path %s", (candidate, expected) => {
    expect(safeRelativeReturnPath(candidate)).toBe(expected);
  });

  it("sends incomplete accounts to onboarding and established sellers to their safe destination", () => {
    expect(
      postAuthenticationPath(
        { sellerId: null, onboardingComplete: false },
        "/dashboard/products",
      ),
    ).toBe("/dashboard/onboarding");
    expect(
      postAuthenticationPath(
        { sellerId: "sel_123", onboardingComplete: false },
        "/dashboard/products",
      ),
    ).toBe("/dashboard/onboarding");
    expect(
      postAuthenticationPath(
        { sellerId: "sel_123", onboardingComplete: true },
        "/dashboard/products",
      ),
    ).toBe("/dashboard/products");
  });

  it("requires an exact same-origin request and matching CSRF values", () => {
    expect(
      validateCsrfRequest({
        allowedOrigin: "http://localhost:3000",
        cookieToken: "csrf-token",
        headerToken: "csrf-token",
        origin: "http://localhost:3000",
      }),
    ).toEqual({ ok: true });
    expect(
      validateCsrfRequest({
        allowedOrigin: "http://localhost:3000",
        cookieToken: "csrf-token",
        headerToken: "different-token",
        origin: "http://localhost:3000",
      }),
    ).toEqual({ ok: false, error: "Invalid security token." });
    expect(
      validateCsrfRequest({
        allowedOrigin: "http://localhost:3000",
        cookieToken: "csrf-token",
        headerToken: "csrf-token",
        origin: "https://attacker.example",
      }),
    ).toEqual({ ok: false, error: "Invalid request origin." });
  });
});
