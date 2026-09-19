import { beforeEach, describe, expect, it, vi } from "vitest";
import { verifySellerTestPurchase } from "@/features/onboarding/controller";
import { authenticatedSellerId } from "@/features/auth/server/authorization";
import { requestAgentPay } from "@/lib/agentpay-api";

vi.mock("@/features/auth/server/authorization", () => ({
  authenticatedSellerId: vi.fn(),
  sellerSessionRequired: vi.fn(),
}));
vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
  updateCurrentSellerPrincipal: vi.fn(),
}));
vi.mock("@/lib/agentpay-api", () => ({ requestAgentPay: vi.fn() }));

const transactionId = "txn_01ARZ3NDEKTSV4RRFFQ69G5FB0";

describe("seller test-purchase verification", () => {
  beforeEach(() => {
    vi.mocked(authenticatedSellerId).mockResolvedValue(
      "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
    );
    vi.mocked(requestAgentPay).mockReset();
  });

  it("records onboarding only after receipt, evidence, and one dashboard row agree", async () => {
    vi.mocked(requestAgentPay)
      .mockResolvedValueOnce({
        ok: true,
        value: {
          transaction: {
            transactionId,
            sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
            status: "FULFILLED",
            commerceLifecycle: { fulfillmentState: "succeeded" },
          },
          evidence: {
            valid: true,
            events: [
              { eventId: "evt_1", eventType: "proxy.forwarded" },
              { eventId: "evt_2", eventType: "delivery.succeeded" },
            ],
          },
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: {
          schemaVersion: "2",
          transaction: { transactionId },
          evidence: { verified: true, eventCount: 2 },
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: { items: [{ transactionId }] },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: {
          sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
          complete: true,
          steps: [],
          publication: { allowed: true, blockers: [] },
          version: 4,
        },
      });

    await expect(verifySellerTestPurchase({ transactionId })).resolves.toEqual({
      ok: true,
      value: expect.objectContaining({
        transactionId,
        dashboardOccurrenceCount: 1,
        evidenceEventCount: 2,
        evidenceValid: true,
        fulfillmentExactlyOnce: true,
        receiptAvailable: true,
      }),
    });
    expect(requestAgentPay).toHaveBeenLastCalledWith(
      "/v1/me/onboarding/sandbox-purchases",
      { method: "POST", body: { transactionId } },
    );
  });

  it("fails closed and never records onboarding when reconciliation is incomplete", async () => {
    vi.mocked(requestAgentPay)
      .mockResolvedValueOnce({
        ok: true,
        value: {
          transaction: {
            transactionId,
            sellerId: "sel_01ARZ3NDEKTSV4RRFFQ69G5FAV",
            status: "FULFILLED",
            commerceLifecycle: { fulfillmentState: "succeeded" },
          },
          evidence: { valid: false, events: [] },
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: {
          schemaVersion: "2",
          transaction: { transactionId },
          evidence: { verified: false, eventCount: 0 },
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: { items: [{ transactionId }, { transactionId }] },
      });

    const result = await verifySellerTestPurchase({ transactionId });
    expect(result).toMatchObject({
      ok: false,
      code: "test_purchase_unverified",
    });
    expect(requestAgentPay).toHaveBeenCalledTimes(3);
  });
});
