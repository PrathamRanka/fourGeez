"use client";

import { DiaTextReveal } from "@/components/ui/dia-text-reveal";

const HEADLINE = "Sell to agents. Settle on-chain.";

export function HeroHeadline() {
  return (
    <h1 id="hero-title">
      <span className="sr-only">{HEADLINE}</span>
      <DiaTextReveal
        aria-hidden="true"
        className="hero-headline-line"
        colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
        data-testid="hero-headline-reveal"
        duration={3}
        startOnView={false}
        text="Sell to agents."
        textColor="#f8f8fb"
      />
      <DiaTextReveal
        aria-hidden="true"
        className="hero-headline-line"
        colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
        data-testid="hero-headline-reveal"
        delay={0.05}
        duration={3.2}
        startOnView={false}
        text="Settle on-chain."
        textColor="#f8f8fb"
      />
    </h1>
  );
}
