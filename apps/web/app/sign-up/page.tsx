import type { Metadata } from "next";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { PublicInfoPage } from "@/features/marketing/view/public-info-page";

export const metadata: Metadata = {
  title: "Start selling",
};

// SignUpPage presents the early-access entry state without pretending account creation is live.
export default function SignUpPage() {
  return (
    <PublicInfoPage
      eyebrow="Seller early access"
      title="Open your AgentPay storefront."
      summary="Seller registration follows with the dashboard. This preview does not submit or retain personal information."
    >
      <div className="grid gap-5 rounded-3xl border border-border bg-card p-[clamp(1.5rem,5vw,3rem)] shadow-[var(--shadow-panel)] sm:grid-cols-2">
        <div>
          <p className="section-kicker">What you prepare</p>
          <h2 className="mt-3">An existing HTTPS service and a repository.</h2>
        </div>
        <div>
          <p>
            You will create a project, verify a payment destination, connect your coding agent, and
            approve every generated route before it becomes purchasable.
          </p>
          <Link href="/docs" className={buttonVariants({ size: "lg", className: "mt-6 h-11 px-4" })}>
            See how setup works
          </Link>
        </div>
      </div>
    </PublicInfoPage>
  );
}
