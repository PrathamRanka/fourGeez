"use client";

import { DiaTextReveal } from "@/components/ui/dia-text-reveal";
import { useHomepageReady } from "@/features/marketing/view/homepage-experience";

const HEADLINE = "Sell to agents. Settle on-chain.";

export function HeroHeadline() {
  const isReady = useHomepageReady();

  return (
    <h1 id="hero-title">
      <span className={isReady ? "sr-only" : undefined}>{HEADLINE}</span>
      {isReady ? (
        <DiaTextReveal
          aria-hidden="true"
          colors={["#2979ff", "#ff5aa5", "#ff6d00"]}
          data-testid="hero-headline-reveal"
          duration={1.1}
          startOnView={false}
          text={HEADLINE}
          textColor="#f8f8fb"
        />
      ) : null}
    </h1>
  );
}
