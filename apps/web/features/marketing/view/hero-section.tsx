import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { Globe } from "@/components/ui/cobe-globe";
import { buttonVariants } from "@/components/ui/button";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";

const commerceMarkers = [
  { location: [37.7749, -122.4194] as [number, number], size: 0.025 },
  { location: [51.5072, -0.1276] as [number, number], size: 0.022 },
  { location: [1.3521, 103.8198] as [number, number], size: 0.024 },
  { location: [19.076, 72.8777] as [number, number], size: 0.027 },
  { location: [-33.8688, 151.2093] as [number, number], size: 0.021 },
];

// HeroSection introduces the product promise with AgentPay's live commerce globe.
export function HeroSection() {
  return (
    <section className="hero-surface" aria-labelledby="hero-title">
      <div className="site-container hero-content">
        <div className="hero-theme-control">
          <ThemeCycleButton />
        </div>
        <div className="hero-copy">
          <h1 id="hero-title" className="hero-title">
            Sell your API to AI agents.
          </h1>
          <p className="hero-description">
            Give your coding agent one key, one prompt, and your approval.
            AgentPay prepares the storefront, payment gate, discovery files, and
            verification for your existing API.
          </p>
          <Link
            href="/sign-up"
            className={buttonVariants({ size: "lg", className: "hero-action" })}
          >
            Get started
            <ArrowRight aria-hidden="true" />
          </Link>
        </div>

        <div className="hero-globe">
          <div className="hero-globe-halo" aria-hidden="true" />
          <Globe markers={commerceMarkers} label="AgentPay commerce network" />
        </div>
      </div>
    </section>
  );
}
