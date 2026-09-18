import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

// ClosingCallToAction gives sellers one clear next step after reviewing the product.
export function ClosingCallToAction() {
  return (
    <section className="closing-surface border-t border-border py-[var(--section-space)]">
      <div className="site-container text-center">
        <p className="section-kicker">Your API already has value</p>
        <h2 className="mx-auto mt-5 max-w-[13ch] font-display text-[clamp(3rem,7vw,6.5rem)] font-semibold leading-[0.94] tracking-[-0.07em]">
          Make it purchasable by the next customer—human or agent.
        </h2>
        <p className="section-copy mx-auto mt-6 max-w-xl">
          Connect a repository, review the generated integration, verify your wallet, and publish
          only when you are ready.
        </p>
        <Link
          href="/sign-up"
          className={buttonVariants({ size: "lg", className: "mt-8 h-12 px-5 text-base" })}
        >
          Launch your storefront
          <ArrowRight aria-hidden="true" />
        </Link>
      </div>
    </section>
  );
}
