import type { Metadata } from "next";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Sign in",
};

// SignInPage provides a truthful account entry state before dashboard authentication ships.
export default function SignInPage() {
  return (
    <PublicInfoPage
      eyebrow="Seller access"
      title="Welcome back."
      summary="Account sign-in is not enabled in this public-site preview. No credentials are collected or stored here."
    >
      <div className="rounded-3xl border border-border bg-card p-[clamp(1.5rem,5vw,3rem)] shadow-[var(--shadow-panel)]">
        <h2>Preparing the seller workspace</h2>
        <p>
          The next product milestone connects verified authentication to onboarding, wallet setup,
          product publishing, and Revenue Lens.
        </p>
        <Link href="/docs" className={buttonVariants({ size: "lg", className: "mt-6 h-11 px-4" })}>
          Review the integration guide
        </Link>
      </div>
    </PublicInfoPage>
  );
}
