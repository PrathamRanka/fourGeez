import type { Metadata } from "next";
import { LegalPage } from "./legal-surface";

export const metadata: Metadata = { title: "Privacy" };

const sections = [
  [
    "What this preview stores",
    "The current public site provides product information only. Seller authentication submits account details only to the same-origin identity boundary.",
  ],
  [
    "What AgentPay avoids",
    "AgentPay does not log authorization headers, payment signatures, wallet material, or seller secrets. Evidence storage is restricted to allowlisted metadata, hashes, and references required to verify a transaction.",
  ],
  [
    "Before production",
    "Retention periods, data subject rights, subprocessors, and contact details will be published before production accounts or real-money operation are enabled.",
  ],
] as const;

export default function PrivacyPage() {
  return (
    <LegalPage
      indexLabel="Data handling index"
      eyebrow="Privacy"
      title="Privacy by minimization."
      summary="Collect only what the transaction and security boundary require."
      sections={sections}
    />
  );
}
