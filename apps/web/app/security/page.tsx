import type { Metadata } from "next";
import { LegalPage, type LegalSection } from "@/app/privacy/legal-surface";
import { buildSupportingPageMetadata } from "@/features/marketing/seo";

export const metadata: Metadata = buildSupportingPageMetadata({
  title: "Security",
  description:
    "The implemented and planned security boundaries for the AgentPay pre-launch x402 service.",
  path: "/security",
});

const sections: readonly LegalSection[] = [
  {
    id: "posture",
    title: "Current security posture",
    paragraphs: [
      "AgentPay is not yet production-ready. The Lean V1 application has been verified locally, while AWS runtime deployment and deployed release verification remain incomplete. Current payment support is limited to mock and x402 testnet operation.",
      "The controls below describe the implemented local product boundary and deployed infrastructure foundations. They are not a certification, warranty, penetration-test result, or substitute for an external application and infrastructure security review.",
    ],
  },
  {
    id: "transactions",
    title: "Transaction integrity",
    paragraphs: [
      "Purchase intents are immutable after creation. AgentPay freezes the seller quote, treats the buyer maximum as a ceiling, and checks the exact amount, asset, network, destination, request, and resource before fulfillment.",
    ],
    bullets: [
      "Unique payment identifiers and conditional writes protect against proof replay.",
      "Only finalized payments can enter the exactly-once seller-forwarding claim.",
      "A model recommendation never authorizes a purchase or bypasses domain validation.",
      "Browser and software-agent purchases share the same authoritative commerce rules.",
    ],
  },
  {
    id: "custody",
    title: "Funds, wallets, and secrets",
    paragraphs: [
      "AgentPay does not custody buyer funds or seller revenue. Sellers prove control of a public payment destination, while AgentPay never requests or stores wallet private keys or seed phrases.",
      "Raw payment proofs, ownership signatures, authorization headers, cookies, confirmation grants, execution capabilities, and seller secrets are excluded from evidence and logs. Where verification requires persistence, AgentPay stores bounded hashes, references, and allowlisted metadata instead.",
    ],
  },
  {
    id: "seller-access",
    title: "Seller and coding-agent access",
    paragraphs: [
      "Seller-hosted code and repository content start untrusted. Project credentials are seller-scoped bootstrap credentials for the required local connector; ordinary cloud MCP requests use short-lived, scoped capabilities and recheck credential and entitlement state.",
      "Commercial MCP mutations require a separate one-time confirmation grant created through the authenticated seller boundary and bound to the exact tool, target, arguments, resource version, credential, seller, and expiry. A coding agent cannot confirm its own proposal.",
    ],
  },
  {
    id: "network",
    title: "Seller endpoint protection",
    paragraphs: [
      "Seller endpoints and webhook destinations are restricted to public HTTPS targets. The service is designed to reject private, loopback, link-local, metadata, redirect, IPv6, and DNS-rebinding SSRF paths and to enforce request, response, and timeout limits.",
      "Production seller fulfillment uses a short-lived cloud-signed execution capability bound to one finalized transaction, seller, route, method, path, and request-body hash. Seller middleware verifies the capability and uses the transaction identifier as its fulfillment idempotency key.",
    ],
  },
  {
    id: "evidence",
    title: "Evidence and auditability",
    paragraphs: [
      "Transaction evidence is append-only, hash-chained, and signed. Evidence payloads contain allowlisted facts and hashes rather than raw payment credentials or unrestricted seller responses.",
      "The Mumbai development infrastructure includes a versioned, encrypted Object Lock evidence bucket and non-exportable asymmetric signing keys. Production backup, restore, retention, deletion, and disaster-recovery procedures still require completion and review.",
    ],
  },
  {
    id: "remaining-work",
    title: "Required before real funds",
    paragraphs: [
      "Real-money processing must remain disabled until the production runtime and release gates pass and the unresolved security obligations are completed.",
    ],
    bullets: [
      "External application and infrastructure security review, including tenant-isolation testing.",
      "Incident-response, key-compromise, backup, restore, and disaster-recovery exercises.",
      "Dependency, container, and infrastructure-as-code scanning in continuous integration.",
      "A documented x402 facilitator service level and failure model.",
      "External legal review covering payment role, refunds, sanctions, privacy, and retention.",
    ],
  },
  {
    id: "reporting",
    title: "Security reporting",
    paragraphs: [
      "A dedicated security reporting address and coordinated disclosure process have not yet been published. Do not include credentials, wallet material, payment proofs, or personal data in an unsolicited report.",
      "The production service must publish an authenticated reporting channel and response process before handling real funds. Until then, review the public technical documentation for the current architecture and limitations.",
    ],
  },
];

export default function SecurityPage() {
  return (
    <LegalPage
      currentPath="/security"
      indexLabel="Security control index"
      eyebrow="Security"
      title="Verification before execution."
      summary="The security boundary is designed around explicit seller authority, exact payment verification, non-custodial settlement, and evidence that can be independently checked."
      notice="This is a pre-launch security statement, not a certification or legal guarantee. AgentPay currently supports local mock and x402 testnet operation. AWS runtime deployment, deployed verification, external security review, and external legal review remain required before real funds."
      sections={sections}
    />
  );
}
