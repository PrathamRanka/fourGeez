import Link from "next/link";
import { ArrowRight, CheckCircle2, Sparkles } from "lucide-react";
import { Globe } from "@/components/ui/cobe-globe";
import { buttonVariants } from "@/components/ui/button";

const commerceMarkers = [
  { location: [37.7749, -122.4194] as [number, number], size: 0.07 },
  { location: [51.5072, -0.1276] as [number, number], size: 0.06 },
  { location: [1.3521, 103.8198] as [number, number], size: 0.065 },
  { location: [19.076, 72.8777] as [number, number], size: 0.075 },
  { location: [-33.8688, 151.2093] as [number, number], size: 0.055 },
];

// HeroSection introduces the product promise and its live commerce network.
export function HeroSection() {
  return (
    <section className="hero-surface relative border-b border-border" aria-labelledby="hero-title">
      <div className="site-container grid min-h-[calc(100svh-4.5rem)] items-center gap-12 py-[clamp(4.5rem,10vw,8.5rem)] lg:grid-cols-[1.08fr_0.92fr]">
        <div className="relative z-10 max-w-[46rem]">
          <div className="hero-kicker inline-flex items-center gap-2 rounded-full border border-primary/15 bg-white/65 px-3 py-1.5 font-mono text-[0.6875rem] font-medium uppercase tracking-[0.13em] text-primary shadow-sm backdrop-blur">
            <Sparkles className="size-3.5" aria-hidden="true" />
            Commerce infrastructure for the agent web
          </div>
          <h1
            id="hero-title"
            className="mt-7 max-w-[14ch] font-display text-[clamp(3.25rem,7.4vw,7rem)] font-semibold leading-[0.94] tracking-[-0.072em] text-foreground"
          >
            Turn your API into a storefront agents can buy from.
          </h1>
          <p className="mt-7 max-w-[40rem] text-[clamp(1.05rem,2vw,1.25rem)] leading-8 text-muted-foreground">
            Start with one key and one prompt. Your coding agent prepares the products, payment
            gates, discovery files, verification, and tests—ready for your approval.
          </p>
          <div className="mt-8 flex flex-col gap-3 sm:flex-row">
            <Link
              href="/sign-up"
              className={buttonVariants({ size: "lg", className: "h-12 px-5 text-base" })}
            >
              Start selling with AgentPay
              <ArrowRight aria-hidden="true" />
            </Link>
            <Link
              href="/docs"
              className={buttonVariants({
                variant: "outline",
                size: "lg",
                className: "h-12 bg-white/55 px-5 text-base backdrop-blur",
              })}
            >
              Read the integration guide
            </Link>
          </div>
          <ul className="mt-8 flex flex-wrap gap-x-5 gap-y-2 text-sm text-muted-foreground">
            {["No custody", "No private keys", "Seller-approved publishing"].map((promise) => (
              <li key={promise} className="flex items-center gap-2">
                <CheckCircle2 className="size-4 text-emerald-600" aria-hidden="true" />
                {promise}
              </li>
            ))}
          </ul>
        </div>

        <div className="hero-globe relative mx-auto w-full max-w-[42rem]">
          <div className="absolute inset-[12%] rounded-full bg-primary/10 blur-3xl" aria-hidden="true" />
          <Globe markers={commerceMarkers} />
          <div className="network-note network-note-top" aria-hidden="true">
            <span className="network-status-dot" />
            PRODUCT DISCOVERED
          </div>
          <div className="network-note network-note-bottom" aria-hidden="true">
            <span className="font-semibold text-foreground">0.08 USDC</span>
            <span>VERIFIED</span>
          </div>
        </div>
      </div>
    </section>
  );
}
