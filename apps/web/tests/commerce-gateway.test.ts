import { afterEach, describe, expect, it } from "vitest";
import { browserPurchaseCookieNames } from "@/features/commerce/commerce-cookie-policy";

describe("commerce gateway browser cookies", () => {
  const originalEnvironment = process.env.AGENTPAY_ENV;

  afterEach(() => {
    if (originalEnvironment === undefined) delete process.env.AGENTPAY_ENV;
    else process.env.AGENTPAY_ENV = originalEnvironment;
  });

  it("uses HTTP-safe local names without weakening production host cookies", () => {
    process.env.AGENTPAY_ENV = "local";
    expect(browserPurchaseCookieNames()).toEqual({
      purchase: "agentpay_purchase",
      csrf: "agentpay_purchase_csrf",
    });

    process.env.AGENTPAY_ENV = "production";
    expect(browserPurchaseCookieNames()).toEqual({
      purchase: "__Host-agentpay_purchase",
      csrf: "__Host-agentpay_purchase_csrf",
    });
  });
});
