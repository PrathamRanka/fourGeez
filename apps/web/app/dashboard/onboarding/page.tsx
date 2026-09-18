import type { Metadata } from "next";
import {
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
    : { paymentDestinations: [], credentials: [] };
  const initialSnapshot: OnboardingSnapshot = {
    seller: storefront,
    ...resources,
  };
  return (
    <SellerOnboarding
      actions={onboardingActions}
      apiOrigin={process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080"}
      initialSnapshot={initialSnapshot}
    />
  );
}

const onboardingActions = {
  createStorefront,
  preparePaymentDestination,
  verifyPaymentDestination,
  createIntegrationCredential,
};
