import type { Metadata } from "next";
import Link from "next/link";
import { RecoveryForms } from "@/features/auth/view/auth-forms";
import { AuthSurface } from "@/features/auth/view/auth-surface";
import styles from "@/features/auth/view/auth-surface.module.css";

export const metadata: Metadata = { title: "Recover account" };

export default function RecoveryPage() {
  return (
    <AuthSurface
      eyebrow="Account recovery"
      title="Restore access safely."
      highlight="Recovery protocol"
      summary="Request a one-time code, choose a new password, and sign in again through a fresh seller session."
    >
      <RecoveryForms />
      <p className={styles.inlineLinks}>
        <Link href="/sign-in">Return to sign in</Link>
      </p>
    </AuthSurface>
  );
}
