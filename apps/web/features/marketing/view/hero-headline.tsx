"use client";

import { DiaTextReveal } from "@/components/ui/dia-text-reveal";

const HEADLINE = "Sell to agents. Settle on-chain.";

export function HeroHeadline() {
  return (
    <h1 id="hero-title">
      <span className="sr-only">{HEADLINE}</span>
      <span className="hero-headline-line" aria-hidden="true">
        <span>Sell to agents.</span>
        <DiaTextReveal
          className="hero-headline-effect"
          colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
          data-testid="hero-headline-reveal"
          duration={1.8}
          startOnView={false}
          text="Sell to agents."
          textColor="#f8f8fb"
        />
      </span>
      <span className="hero-headline-line" aria-hidden="true">
        <span>Settle on-chain.</span>
        <DiaTextReveal
          className="hero-headline-effect"
          colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
          delay={0.45}
          duration={2}
          startOnView={false}
          text="Settle on-chain."
          textColor="#f8f8fb"
        />
      </span>
    </h1>
  );
}
