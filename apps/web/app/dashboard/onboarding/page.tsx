import type { Metadata } from "next";
import {
  activateSellerService,
  createIntegrationCredential,
  createStorefront,
  preparePaymentDestination,
  verifyPaymentDestination,
} from "@/features/onboarding/controller";
import { listOnboardingResources } from "@/features/onboarding/controller";
import { getSellerSession } from "@/features/auth/server/session";
import type { OnboardingSnapshot } from "@/features/onboarding/model";
import { SellerOnboarding } from "@/features/onboarding/view/seller-onboarding";

export const metadata: Metadata = {
  title: "Seller onboarding",
};

// OnboardingPage starts a secure seller setup session without serializing server credentials.
export default async function OnboardingPage() {
  const session = await getSellerSession();
  const storefront = session?.principal.storefront ?? null;
  const resources = storefront
    ? await listOnboardingResources()
    : {
        paymentDestinations: [],
        credentials: [],
        onboarding: {
          sellerId: null,
          complete: false,
          currentStep: "storefront_created" as const,
          steps: [],
          publication: { allowed: false, blockers: [] },
          version: 0,
        },
      };
  const initialSnapshot: OnboardingSnapshot = {
    seller: storefront,
    ...resources,
  };
  return (
    <SellerOnboarding
      actions={onboardingActions}
      apiOrigin={`${(process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000").replace(/\/$/, "")}/api/backend`}
      initialSnapshot={initialSnapshot}
    />
  );
}

const onboardingActions = {
  createStorefront,
  activateSellerService,
  preparePaymentDestination,
  verifyPaymentDestination,
  createIntegrationCredential,
};
