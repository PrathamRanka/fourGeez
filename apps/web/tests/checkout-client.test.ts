import { afterEach, describe, expect, it, vi } from "vitest";
import { requestCheckout } from "@/features/commerce/checkout-client";

describe("checkout client recovery", () => {
  afterEach(() => vi.unstubAllGlobals());

  it.each(["payment_expired", "route_contract_stale"])(
    "forces a new checkout for %s even when details are omitted",
    async (code) => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue(
          new Response(
            JSON.stringify({
              error: {
                code,
                message: "The payment contract is no longer current.",
                requestId: "req_checkout_recovery",
              },
            }),
            {
              status: code === "payment_expired" ? 410 : 409,
              headers: { "Content-Type": "application/json" },
            },
          ),
        ),
      );

      await expect(
        requestCheckout("/api/commerce/complete", {}),
      ).resolves.toEqual({
        ok: false,
        error: "The payment contract is no longer current.",
        retryable: true,
        recoveryAction: "start_new_checkout",
      });
    },
  );
});
