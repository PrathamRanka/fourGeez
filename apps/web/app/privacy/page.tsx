import type { Metadata } from "next";
import { LegalPage, type LegalSection } from "./legal-surface";

export const metadata: Metadata = {
  title: "Privacy",
  description:
    "How the AgentPay pre-launch service handles seller, transaction, session, and evidence data.",
  alternates: {
    canonical: "https://agentpay.prathamranka.in/privacy",
  },
};

const sections: readonly LegalSection[] = [
  {
    id: "scope",
    title: "Current scope",
    paragraphs: [
      "AgentPay is a pre-launch development service. The current product is intended for local mock payments and x402 testnet use, not production accounts or real-money operation.",
      "This notice describes the data boundaries implemented in the current codebase and the additional privacy work still required before production activation.",
    ],
  },
  {
    id: "data-we-handle",
    title: "Data the service handles",
    paragraphs: [
      "AgentPay handles the minimum information needed to authenticate a seller, configure a storefront, verify an integration, process a test transaction, and preserve an auditable record.",
    ],
    bullets: [
      "Seller account identifiers, storefront profile fields, product configuration, and support or security-notification preferences supplied by the seller.",
      "Public wallet addresses, asset and network identifiers, and bounded wallet-ownership verification metadata. Raw ownership signatures are validated and discarded.",
      "Purchase intent, transaction, fulfillment, dispute, webhook, quota, and audit metadata described in the product data model.",
      "Opaque browser and seller session state, CSRF protections, request identifiers, timestamps, and security events needed to operate and protect the service.",
    ],
  },
  {
    id: "data-we-avoid",
    title: "Data AgentPay avoids",
    paragraphs: [
      "AgentPay does not custody buyer funds and never requests or stores wallet private keys or seed phrases. Buyer funds are designed to settle directly to a seller-controlled payment destination.",
      "The service does not retain raw payment proofs, raw wallet-ownership signatures, authorization headers, cookies, approval tokens, execution capabilities, or seller secrets in evidence records or logs. Evidence is limited to allowlisted facts, hashes, signatures, and safe provider references.",
    ],
    note:
      "Do not submit production wallet credentials, private keys, customer datasets, or unrelated repository secrets to this pre-launch service.",
  },
  {
    id: "use",
    title: "How information is used",
    paragraphs: [
      "Information is used to provide seller authentication and onboarding, publish storefront information, authorize scoped integrations, prevent replay and duplicate fulfillment, reconcile test transactions, produce receipts and evidence, investigate failures, and support disputes.",
      "Repository analysis and generated SEO or agent-discovery work are limited to supported integration inputs and public product metadata. AgentPay is not intended to receive unrelated source files, environment files, credentials, wallet material, customer information, or proprietary datasets.",
    ],
  },
  {
    id: "cookies-analytics",
    title: "Cookies and site analytics",
    paragraphs: [
      "AgentPay uses strictly necessary first-party cookies for seller sessions, CSRF protection, and browser purchase authorization. These cookies support security and requested product flows; they are not advertising cookies.",
      "Vercel Web Analytics is enabled across the site through the pinned @vercel/analytics 2.0.1 package. According to Vercel's current product documentation, its default web analytics does not use third-party cookies and reports aggregated page-view information using a short-lived request-derived visitor hash.",
      "Analytics data may include the event time, visited path, dynamic route, filtered query parameters, referrer, approximate location, operating system, browser, device type, and analytics script version. No custom analytics events are currently configured, and AgentPay does not intentionally send account names, email addresses, wallet material, payment proofs, or seller secrets to analytics.",
    ],
    note:
      "Final counsel review must confirm the consent, notice, subprocessor, retention, and cross-border-transfer requirements for every launch jurisdiction and the exact Vercel project configuration. Sensitive values must never be placed in URLs or analytics events.",
  },
  {
    id: "storage-sharing",
    title: "Storage and service providers",
    paragraphs: [
      "AgentPay uses Vercel for the website and web analytics, and uses infrastructure, identity, and x402 facilitator boundaries to operate the service. Access should be limited to the purpose required by each boundary, and transaction-critical authorization must fail closed when an authoritative dependency is unavailable.",
      "A final production subprocessor list, international-transfer position, operating entity, privacy contact, and region-specific disclosures have not yet been published. Those decisions require external legal review before production accounts or real-money use are enabled.",
    ],
  },
  {
    id: "retention",
    title: "Retention, deletion, and rights",
    paragraphs: [
      "Development operational data may be removed during environment teardown. Evidence is designed to be append-only and protected from casual deletion, which means deletion requests may conflict with integrity and dispute requirements.",
      "Production retention and deletion periods, data-subject request procedures, identity verification for requests, and legally required exceptions are not final. AgentPay will not represent these rights or timelines as complete until external legal review is finished and an operational request channel exists.",
    ],
  },
  {
    id: "changes",
    title: "Changes and contact status",
    paragraphs: [
      "This notice may change as the production architecture, service providers, retention schedule, and legal operating model are finalized. Material changes will receive a new effective date on this page.",
      "A dedicated privacy contact and request process are still pending. Until they are published, this page is product information rather than a complete production privacy notice.",
    ],
  },
];

export default function PrivacyPage() {
  return (
    <LegalPage
      currentPath="/privacy"
      indexLabel="Data handling index"
      eyebrow="Privacy"
      title="Privacy by minimization."
      summary="A plain-language account of what the pre-launch service handles, what it deliberately avoids, and what remains unresolved before production."
      notice="AgentPay is not yet production-ready. This page is an implementation-aligned pre-launch privacy notice, not a finalized policy or legal advice. External legal review, production retention decisions, contact details, and subprocessor disclosures are still required."
      sections={sections}
    />
  );
}
