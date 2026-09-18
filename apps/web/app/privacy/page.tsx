import type { Metadata } from "next";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Privacy",
};

// PrivacyPage states the current data-minimization commitments for the preview.
export default function PrivacyPage() {
  return (
    <PublicInfoPage
      eyebrow="Privacy"
      title="Privacy by minimization."
      summary="This public preview does not collect account, payment, or wallet information. Production data handling will remain purpose-limited and auditable."
    >
      <h2>What this preview stores</h2>
      <p>
        The current public site provides product information only. Its sign-in and sign-up entry
        pages do not submit personal information.
      </p>
      <h2>What AgentPay will avoid</h2>
      <p>
        AgentPay will not log authorization headers, payment signatures, approval tokens, wallet
        material, or seller secrets. Evidence storage is restricted to allowlisted metadata, hashes,
        and references required to verify a transaction.
      </p>
      <h2>Before production</h2>
      <p>
        Retention periods, data subject rights, subprocessors, and contact details will be published
        before production accounts or real-money operation are enabled.
      </p>
    </PublicInfoPage>
  );
}
