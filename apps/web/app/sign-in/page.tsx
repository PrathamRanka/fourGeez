import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { SignInForm } from "@/features/auth/view/auth-forms";
import { AuthSurface } from "@/features/auth/view/auth-surface";
import styles from "@/features/auth/view/auth-surface.module.css";

export const metadata: Metadata = { title: "Sign in" };

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
    <AuthSurface
      eyebrow="Seller access"
      title="Welcome back."
      highlight="After sign in"
      summary="Manage products, payments, and fulfillment from one focused workspace."
    >
      {parameters.verified === "1" ? (
        <p className={styles.developmentCode} role="status">
          Email verified. Sign in to continue.
        </p>
      ) : null}
      {parameters.recovered === "1" ? (
        <p className={styles.developmentCode} role="status">
          Password updated. Sign in with the new password.
        </p>
      ) : null}
      <SignInForm returnTo={returnTo} />
      <div className={styles.inlineLinks}>
        <Link href="/recover">Recover your password</Link>
        <Link href={`/sign-up?returnTo=${encodeURIComponent(returnTo)}`}>
          Create a seller account
        </Link>
      </div>
    </AuthSurface>
  );
}
