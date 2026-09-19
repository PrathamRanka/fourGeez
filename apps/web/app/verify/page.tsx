import type { Metadata } from "next";
import Link from "next/link";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import {
  ResendVerificationForm,
  VerifyForm,
} from "@/features/auth/view/auth-forms";
import { AuthSurface } from "@/features/auth/view/auth-surface";
import styles from "@/features/auth/view/auth-surface.module.css";

export const metadata: Metadata = { title: "Verify email" };

type VerifyPageProps = { searchParams?: Promise<{ returnTo?: string }> };

export default async function VerifyPage({
  searchParams,
}: VerifyPageProps = {}) {
  const parameters = (await searchParams) ?? {};
  return (
    <AuthSurface
      eyebrow="Account verification"
      title="Verify the inbox you control."
      highlight="Email ownership check"
      summary="Verification completes registration but does not create a browser session until you sign in."
    >
      <VerifyForm returnTo={safeRelativeReturnPath(parameters.returnTo)} />
      <ResendVerificationForm />
      <div className={styles.inlineLinks}>
        <span>Code expired?</span>
        <Link href="/sign-up">Return to registration</Link>
      </div>
    </AuthSurface>
  );
}
