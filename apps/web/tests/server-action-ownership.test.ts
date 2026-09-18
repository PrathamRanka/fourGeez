import { beforeEach, describe, expect, it, vi } from "vitest";
import { getSellerSession } from "@/features/auth/server/session";
import { updateCurrentSellerPrincipal } from "@/features/auth/server/session";
import { createIntegrationCredential } from "@/features/onboarding/controller";
import { createDraft } from "@/features/products/controller";
import { requestAgentPay } from "@/lib/agentpay-api";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
  updateCurrentSellerPrincipal: vi.fn(),
}));
vi.mock("@/lib/agentpay-api", () => ({
  requestAgentPay: vi.fn(),
}));

describe("seller server-action ownership", () => {
  beforeEach(() => {
    vi.mocked(getSellerSession).mockResolvedValue({
      sessionId: "opaque-session",
      accessToken: "server-token",
      expiresAt: "2099-09-18T00:00:00Z",
      principal: {
        subject: "owner-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_session_owner",
        onboardingComplete: true,
      },
    });
    vi.mocked(requestAgentPay).mockResolvedValue({ ok: true, value: {} });
  });

  it("uses the session seller for product mutations", async () => {
    await createDraft({
      sellerId: "sel_attacker",
      displayName: "Research Report",
      productSlug: "research-report",
      method: "POST",
      pathPattern: "/research",
      description: "Research",
      mimeType: "application/json",
      amount: "1000000",
      asset: "USDC",
      network: "eip155:84532",
      payTo: "0x1111111111111111111111111111111111111111",
      upstreamTimeoutSeconds: 20,
    });
    expect(requestAgentPay).toHaveBeenCalledWith(
      "/v1/sellers/sel_session_owner/routes",
      expect.anything(),
    );
  });

  it("uses the session seller for credential creation", async () => {
    await createIntegrationCredential({ sellerId: "sel_attacker" });
    expect(requestAgentPay).toHaveBeenCalledWith(
      "/v1/sellers/sel_session_owner/integration-credentials",
      expect.anything(),
    );
  });

  it("persists onboarding completion only after required seller resources exist", async () => {
    vi.mocked(requestAgentPay)
      .mockResolvedValueOnce({
        ok: true,
        value: {
          credentialId: "key_123",
          sellerId: "sel_session_owner",
          label: "Primary coding agent",
          scopes: ["read"],
          version: 1,
          token: "one-time-key",
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: {
          items: [
            {
              destinationId: "dst_123",
              sellerId: "sel_session_owner",
              asset: "USDC",
              network: "eip155:84532",
              address: "0x1111111111111111111111111111111111111111",
              status: "active",
              version: 1,
            },
          ],
        },
      })
      .mockResolvedValueOnce({
        ok: true,
        value: {
          items: [
            {
              credentialId: "key_123",
              sellerId: "sel_session_owner",
              label: "Primary coding agent",
              scopes: ["read"],
              version: 1,
              revokedAt: null,
            },
          ],
        },
      });

    await createIntegrationCredential({ sellerId: "sel_attacker" });

    expect(updateCurrentSellerPrincipal).toHaveBeenCalledWith({
      sellerId: "sel_session_owner",
      onboardingComplete: true,
    });
  });
});
