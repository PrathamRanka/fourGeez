import type { Metadata } from "next";
import Link from "next/link";
import { RecoveryForms } from "@/features/auth/view/auth-forms";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = { title: "Recover account" };

export default function RecoveryPage() {
  return (
    <PublicInfoPage
      eyebrow="Account recovery"
      title="Restore access safely."
      summary="Request a one-time code, choose a new password, and sign in again through a fresh seller session."
    >
      <RecoveryForms />
      <p className="auth-return-link">
        <Link href="/sign-in">Return to sign in</Link>
      </p>
    </PublicInfoPage>
  );
}
