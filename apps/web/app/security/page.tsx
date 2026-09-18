import type { Metadata } from "next";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Security",
};

// SecurityPage summarizes the controls that protect AgentPay transactions.
export default function SecurityPage() {
  return (
    <PublicInfoPage
      eyebrow="Security"
      title="Verification before execution."
      summary="AgentPay treats payment, model output, seller endpoints, and every network boundary as untrusted until verified."
    >
      <h2>Transaction safety</h2>
      <p>
        Purchase intents are immutable, exact-price payments reject underpayment and overpayment,
        payment identifiers are unique, and exactly one caller may claim seller fulfillment.
      </p>
      <h2>Seller protection</h2>
      <p>
        AgentPay validates destinations, blocks private and metadata network targets, signs
        fulfillment requests, limits response size, and never gives model code direct payment
        capability or seller secrets.
      </p>
      <h2>Key handling</h2>
      <p>
        Sellers prove control of a payment destination. AgentPay never requests or stores wallet
        private keys and avoids retaining raw payment proofs when a safe hash and provider reference
        are sufficient.
      </p>
    </PublicInfoPage>
  );
}
