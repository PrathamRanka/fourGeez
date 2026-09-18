import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

const includedFeatures = [
  "Guided MCP integration",
  "Agent and browser-wallet checkout",
  "Payment and fulfillment evidence",
  "Technical SEO and agent discovery files",
] as const;

// PricingPreview communicates the commercial model without inventing an unapproved price.
export function PricingPreview() {
  return (
    <section id="pricing" className="site-container py-[var(--section-space)]">
      <div className="pricing-panel grid overflow-hidden rounded-[2rem] border border-border bg-card lg:grid-cols-[1fr_0.78fr]">
        <div className="p-[clamp(1.5rem,5vw,4.5rem)]">
          <p className="section-kicker">Simple seller pricing</p>
          <h2 className="section-title mt-4 max-w-[13ch]">
            Pay for the commerce layer, not your own revenue.
          </h2>
          <p className="section-copy mt-5 max-w-xl">
            AgentPay plans combine a seller subscription with optional usage-based fees. Buyer
            payments continue directly to your verified wallet.
          </p>
          <ul className="mt-8 grid gap-3 sm:grid-cols-2">
            {includedFeatures.map((feature) => (
              <li key={feature} className="flex items-start gap-2.5 text-sm text-muted-foreground">
                <Check className="mt-0.5 size-4 shrink-0 text-emerald-600" aria-hidden="true" />
                {feature}
              </li>
            ))}
          </ul>
        </div>
        <div className="flex flex-col justify-between border-t border-border bg-primary/5 p-[clamp(1.5rem,5vw,4rem)] lg:border-t-0 lg:border-l">
          <div>
            <p className="font-mono text-[0.6875rem] uppercase tracking-[0.14em] text-primary">
              Early access
            </p>
            <p className="mt-4 font-display text-4xl font-semibold tracking-[-0.06em]">Launch plan</p>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">
              Pricing is finalized before production billing. Join early access to shape limits and
              support levels.
            </p>
          </div>
          <Link
            href="/sign-up"
            className={buttonVariants({ size: "lg", className: "mt-10 h-12 justify-between px-5" })}
          >
            Request seller access
            <ArrowRight aria-hidden="true" />
          </Link>
        </div>
      </div>
    </section>
  );
}
