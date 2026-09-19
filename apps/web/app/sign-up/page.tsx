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
      title="Start selling to agents."
      highlight="Three simple steps"
      summary="Create your account. We will guide you through wallet setup, products, and launch."
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
