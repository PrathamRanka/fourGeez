import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import OnboardingPage from "@/app/dashboard/onboarding/page";
import { getSellerSession } from "@/features/auth/server/session";
import { listOnboardingResources } from "@/features/onboarding/controller";

vi.mock("@/features/auth/server/session", () => ({
  getSellerSession: vi.fn(),
}));
vi.mock("@/features/onboarding/controller", () => ({
  activateSellerService: vi.fn(),
  createIntegrationCredential: vi.fn(),
  createStorefront: vi.fn(),
  listOnboardingResources: vi.fn(),
  preparePaymentDestination: vi.fn(),
  verifyPaymentDestination: vi.fn(),
}));
vi.mock("@/features/onboarding/view/seller-onboarding", () => ({
  SellerOnboarding: ({
    initialSnapshot,
  }: {
    initialSnapshot: { seller: { sellerId: string } | null };
  }) => (
    <p>
      {initialSnapshot.seller
        ? `Resumed ${initialSnapshot.seller.sellerId}`
        : "New seller"}
    </p>
  ),
}));

describe("resumable seller onboarding", () => {
  it("restores the signed-in owner's storefront and setup resources", async () => {
    vi.mocked(getSellerSession).mockResolvedValue({
      sessionId: "opaque-session",
      accessToken: "server-token",
      expiresAt: "2099-09-18T00:00:00Z",
      principal: {
        subject: "owner-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_session_owner",
        onboardingComplete: false,
        storefront: {
          sellerId: "sel_session_owner",
          name: "Northstar Research",
          slug: "northstar",
          upstreamBaseUrl: "https://seller.example",
          status: "draft",
          version: 1,
        },
      },
    });
    vi.mocked(listOnboardingResources).mockResolvedValue({
      paymentDestinations: [],
      credentials: [],
      onboarding: {
        sellerId: "sel_session_owner",
        complete: false,
        currentStep: "payment_destination_verified",
        steps: [],
        publication: { allowed: false, blockers: [] },
        version: 1,
      },
    });

    render(await OnboardingPage());

    expect(listOnboardingResources).toHaveBeenCalledWith();
    expect(screen.getByText("Resumed sel_session_owner")).toBeVisible();
  });
});
