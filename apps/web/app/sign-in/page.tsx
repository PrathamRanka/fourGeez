import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { SignInForm } from "@/features/auth/view/auth-forms";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Sign in",
};

type SignInPageProps = {
  searchParams?: Promise<{
    recovered?: string;
    returnTo?: string;
    verified?: string;
  }>;
};

export default async function SignInPage({
  searchParams,
}: SignInPageProps = {}) {
  const parameters = (await searchParams) ?? {};
  const returnTo = safeRelativeReturnPath(parameters.returnTo);
  return (
    <PublicInfoPage
      eyebrow="Seller access"
      title="Return to your storefront."
      summary="Sign in through AgentPay’s same-origin session boundary. Your seller identity never comes from a dashboard URL."
    >
      <div className="auth-page-grid">
        <SignInForm returnTo={returnTo} />
        <aside className="auth-aside" aria-label="Account help">
          {parameters.verified === "1" ? (
            <p role="status">Email verified. Sign in to continue.</p>
          ) : null}
          {parameters.recovered === "1" ? (
            <p role="status">
              Password updated. Sign in with the new password.
            </p>
          ) : null}
          <h2>Need account help?</h2>
          <p>
            Recover access without placing session credentials in links or
            browser storage.
          </p>
          <Link href="/recover">Recover your password</Link>
          <Link href={`/sign-up?returnTo=${encodeURIComponent(returnTo)}`}>
            Create a seller account
          </Link>
        </aside>
      </div>
    </PublicInfoPage>
  );
}
