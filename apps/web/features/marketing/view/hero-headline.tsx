"use client";

import { DiaTextReveal } from "@/components/ui/dia-text-reveal";

const HEADLINE = "Sell to agents. Settle on-chain.";

export function HeroHeadline() {
  return (
    <h1 id="hero-title">
      <span className="sr-only">{HEADLINE}</span>
      <DiaTextReveal
        aria-hidden="true"
        colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
        data-testid="hero-headline-reveal"
        duration={1.1}
        startOnView={false}
        text={HEADLINE}
        textColor="#f8f8fb"
      />
    </h1>
  );
}
