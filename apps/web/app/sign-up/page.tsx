import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { SignUpForm } from "@/features/auth/view/auth-forms";
import { AuthSurface } from "@/features/auth/view/auth-surface";
import styles from "@/features/auth/view/auth-surface.module.css";

export const metadata: Metadata = { title: "Start selling" };

type SignUpPageProps = { searchParams?: Promise<{ returnTo?: string }> };

export default async function SignUpPage({
  searchParams,
}: SignUpPageProps = {}) {
  const parameters = (await searchParams) ?? {};
  const returnTo = safeRelativeReturnPath(parameters.returnTo);
  return (
    <AuthSurface
      eyebrow="Seller registration"
      title="Open your AgentPay storefront."
      highlight="Account and storefront controls"
      summary="Create one verified owner account, then resume setup from the first incomplete storefront step."
    >
      <SignUpForm returnTo={returnTo} />
      <div className={styles.inlineLinks}>
        <span>Already registered?</span>
        <Link href={`/sign-in?returnTo=${encodeURIComponent(returnTo)}`}>
          Sign in
        </Link>
      </div>
    </AuthSurface>
  );
}
