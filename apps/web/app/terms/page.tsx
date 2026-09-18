import type { Metadata } from "next";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Terms",
};

// TermsPage communicates the preview status and current commercial boundaries.
export default function TermsPage() {
  return (
    <PublicInfoPage
      eyebrow="Terms"
      title="Clear terms for an early product."
      summary="AgentPay is currently a development preview. These notes describe product boundaries and are not the final production terms of service."
    >
      <h2>Preview availability</h2>
      <p>
        The public site demonstrates the intended AgentPay experience. Account access, production
        billing, service levels, and real-money availability are not offered through this preview.
      </p>
      <h2>Seller responsibilities</h2>
      <p>
        Sellers remain responsible for the digital service they operate, the accuracy of their
        products and prices, their payment destination, and approving generated repository or
        production changes.
      </p>
      <h2>Payment boundary</h2>
      <p>
        In V1, buyer funds settle directly to the seller. AgentPay does not custody or redistribute
        seller revenue. Final subscription, usage, dispute, and liability terms will be published
        before commercial launch.
      </p>
    </PublicInfoPage>
  );
}
