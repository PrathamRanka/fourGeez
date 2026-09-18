import type { Metadata } from "next";
import {
  createIntegrationCredential,
  createStorefront,
  preparePaymentDestination,
  verifyPaymentDestination,
} from "@/features/onboarding/controller";
import type { OnboardingSnapshot } from "@/features/onboarding/model";
import { SellerOnboarding } from "@/features/onboarding/view/seller-onboarding";

export const metadata: Metadata = {
  title: "Seller onboarding",
};

const emptySnapshot: OnboardingSnapshot = {
  seller: null,
  paymentDestinations: [],
  credentials: [],
};

// OnboardingPage starts a secure seller setup session without serializing server credentials.
export default function OnboardingPage() {
  return (
    <SellerOnboarding
      actions={onboardingActions}
      apiOrigin={process.env.AGENTPAY_API_ORIGIN ?? "http://localhost:8080"}
      initialSnapshot={emptySnapshot}
    />
  );
}

const onboardingActions = {
  createStorefront,
  preparePaymentDestination,
  verifyPaymentDestination,
  createIntegrationCredential,
};
