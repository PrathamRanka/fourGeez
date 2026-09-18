import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { SignUpForm } from "@/features/auth/view/auth-forms";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Start selling",
};

type SignUpPageProps = { searchParams?: Promise<{ returnTo?: string }> };

export default async function SignUpPage({
  searchParams,
}: SignUpPageProps = {}) {
  const parameters = (await searchParams) ?? {};
  const returnTo = safeRelativeReturnPath(parameters.returnTo);
  return (
    <PublicInfoPage
      eyebrow="Seller registration"
      title="Open your AgentPay storefront."
      summary="Create one verified owner account, then resume setup from the first incomplete storefront step."
    >
      <div className="auth-page-grid">
        <SignUpForm returnTo={returnTo} />
        <aside className="auth-aside" aria-label="Registration details">
          <h2>One owner for Lean V1</h2>
          <p>
            Your account controls storefront setup, products, credentials, and
            seller billing.
          </p>
          <p>Already registered?</p>
          <Link href={`/sign-in?returnTo=${encodeURIComponent(returnTo)}`}>
            Sign in
          </Link>
        </aside>
      </div>
    </PublicInfoPage>
  );
}
