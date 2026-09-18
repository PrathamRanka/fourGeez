import type { Metadata } from "next";
import { LegalPage } from "@/app/privacy/legal-surface";

export const metadata: Metadata = { title: "Terms" };

const sections = [
  [
    "Preview availability",
    "The public site demonstrates the intended AgentPay experience. Production billing, service levels, and real-money availability are not offered through this preview.",
  ],
  [
    "Seller responsibilities",
    "Sellers remain responsible for the digital service they operate, product accuracy, prices, payment destinations, and approval of repository or production changes.",
  ],
  [
    "Payment boundary",
    "Buyer funds settle directly to the seller in V1. AgentPay does not custody or redistribute seller revenue. Final commercial terms will be published before launch.",
  ],
] as const;

export default function TermsPage() {
  return (
    <LegalPage
      indexLabel="Commercial boundary index"
      eyebrow="Terms"
      title="Clear terms for an early product."
      summary="A development preview with explicit seller and payment boundaries."
      sections={sections}
    />
  );
}
