import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { VerifyForm } from "@/features/auth/view/auth-forms";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = { title: "Verify email" };

type VerifyPageProps = { searchParams?: Promise<{ returnTo?: string }> };

export default async function VerifyPage({
  searchParams,
}: VerifyPageProps = {}) {
  const parameters = (await searchParams) ?? {};
  return (
    <PublicInfoPage
      eyebrow="Account verification"
      title="Verify the inbox you control."
      summary="Verification completes registration but does not create a browser session until you sign in."
    >
      <div className="auth-page-grid">
        <VerifyForm returnTo={safeRelativeReturnPath(parameters.returnTo)} />
        <aside className="auth-aside" aria-label="Verification help">
          <h2>Code expired?</h2>
          <p>
            Start registration again to issue a new bounded verification
            challenge.
          </p>
          <Link href="/sign-up">Return to registration</Link>
        </aside>
      </div>
    </PublicInfoPage>
  );
}
