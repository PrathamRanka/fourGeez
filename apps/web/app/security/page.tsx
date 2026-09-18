import type { Metadata } from "next";
import { LegalPage } from "@/app/privacy/legal-surface";

export const metadata: Metadata = { title: "Security" };

const sections = [
  [
    "Transaction safety",
    "Purchase intents are immutable, exact-price payments reject underpayment and overpayment, payment identifiers are unique, and exactly one caller may claim seller fulfillment.",
  ],
  [
    "Seller protection",
    "AgentPay validates destinations, blocks unsafe network targets, signs fulfillment requests, limits response size, and never gives model code direct payment capability or seller secrets.",
  ],
  [
    "Key handling",
    "Sellers prove control of a payment destination. AgentPay never requests or stores wallet private keys and retains safe hashes and provider references instead of raw payment proofs.",
  ],
] as const;

export default function SecurityPage() {
  return (
    <LegalPage
      indexLabel="Security control index"
      eyebrow="Security"
      title="Verification before execution."
      summary="Every payment, model output, seller endpoint, and network boundary starts untrusted."
      sections={sections}
    />
  );
}
