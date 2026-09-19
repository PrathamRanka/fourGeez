import type { Metadata } from "next";
import { LegalPage, type LegalSection } from "@/app/privacy/legal-surface";
import { buildSupportingPageMetadata } from "@/features/marketing/seo";

export const metadata: Metadata = buildSupportingPageMetadata({
  title: "Terms",
  description:
    "Pre-launch terms and operating boundaries for the AgentPay seller and x402 testnet preview.",
  path: "/terms",
});

const sections: readonly LegalSection[] = [
  {
    id: "status",
    title: "Status of these terms",
    paragraphs: [
      "These terms describe the current AgentPay development preview. They are not lawyer-approved final terms, and they do not represent that AgentPay is ready to offer a production paid service.",
      "External legal review is still required for the payment role, privacy, retention, sanctions, refunds, tax, liability, governing law, and any production commercial relationship.",
    ],
  },
  {
    id: "preview",
    title: "Preview availability",
    paragraphs: [
      "Current payment support is mock or x402 testnet only. Production service levels, uptime commitments, seller subscription billing, card checkout, and real-money availability are not offered through this preview.",
      "The preview may change, pause, reset, or remove development data while deployment and release verification remain incomplete. Do not rely on it for production workloads or time-sensitive commercial activity.",
    ],
  },
  {
    id: "accounts",
    title: "Seller accounts and access",
    paragraphs: [
      "A seller is responsible for accurate account and storefront information, protecting project credentials and sessions, using supported integration paths, and promptly revoking access that may be compromised.",
      "AgentPay may deny privileged operations when identity, entitlement, credential, seller status, quota, or a required dependency cannot be verified. The current Lean V1 model supports one seller owner per account.",
    ],
  },
  {
    id: "seller-responsibilities",
    title: "Seller responsibilities",
    paragraphs: [
      "Sellers own and operate the upstream digital service that fulfills each product. They remain responsible for product accuracy, lawful content, pricing, public descriptions, customer support, payment destinations, service performance, and the repository or deployment changes they approve.",
    ],
    bullets: [
      "Do not publish illegal, deceptive, infringing, harmful, or unauthorized products or content.",
      "Do not bypass payment, entitlement, confirmation, replay-protection, or request-verification controls.",
      "Do not submit private keys, seed phrases, production customer data, or unrelated secrets to AgentPay or a connected coding agent.",
      "Review proposed prices, routes, generated code, and production changes before authorizing them.",
    ],
  },
  {
    id: "intellectual-property",
    title: "Product and source rights",
    paragraphs: [
      "AgentPay software, site, documentation, and branding are proprietary except for identified third-party components and independently owned contributions. Access to the service or a public source repository does not grant a broader right to copy, modify, redistribute, commercialize, or use AgentPay marks beyond the applicable repository terms or a separate written agreement.",
      "Sellers retain their rights in their upstream services, product content, and authorized integration inputs. Sellers must have the rights needed for anything they submit, connect, publish, or ask AgentPay and a coding agent to process.",
    ],
  },
  {
    id: "payments",
    title: "Payments and custody",
    paragraphs: [
      "In the current x402 design, buyer funds settle directly to the seller's verified wallet. AgentPay does not custody buyer funds, hold seller revenue, or store wallet private keys.",
      "A seller-approved fixed quote is authoritative. A buyer maximum is a ceiling, not permission to charge a different amount. Testnet payments require an exact amount, asset, network, destination, and resource match before fulfillment.",
    ],
  },
  {
    id: "refunds",
    title: "Disputes and refunds",
    paragraphs: [
      "AgentPay records transaction evidence and applies deterministic dispute classifications to recorded facts. A recommendation is not an automatic transfer of funds.",
      "In Lean V1, AgentPay does not execute, custody, or guarantee refunds. An authenticated seller may record one externally completed full refund for an eligible finalized transaction. The seller-supplied reference is an audit statement, not independent network proof.",
    ],
  },
  {
    id: "availability",
    title: "Availability and unresolved legal terms",
    paragraphs: [
      "The preview is provided for evaluation and development. Final warranty disclaimers, liability limits, indemnities, governing law, dispute-resolution terms, suspension rights, termination effects, and production support obligations remain subject to external legal review.",
      "Do not interpret this page as legal advice or as a complete production contract. A finalized version must identify the operating entity and contact channel before production accounts or real funds are enabled.",
    ],
  },
];

export default function TermsPage() {
  return (
    <LegalPage
      currentPath="/terms"
      indexLabel="Commercial boundary index"
      eyebrow="Terms"
      title="Clear terms for an early product."
      summary="The current operating rules, seller responsibilities, and payment boundaries for AgentPay's pre-launch testnet service."
      notice="These pre-launch terms are an implementation summary, not legal advice and not lawyer-approved final terms. AgentPay is not yet production-ready, and external legal review is required before production accounts, subscriptions, or real-money use."
      sections={sections}
    />
  );
}
