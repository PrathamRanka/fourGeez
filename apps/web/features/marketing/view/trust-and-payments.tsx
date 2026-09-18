import { BadgeCheck, KeyRound, SearchCheck, WalletCards } from "lucide-react";

const trustPoints = [
  {
    icon: WalletCards,
    title: "Direct settlement",
    description:
      "Buyer funds settle directly to your verified wallet for the configured asset and network.",
  },
  {
    icon: KeyRound,
    title: "Keys stay yours",
    description:
      "AgentPay verifies wallet control but never requests, stores, or exposes your private key.",
  },
  {
    icon: BadgeCheck,
    title: "Execution is gated",
    description:
      "Trust Gate validates approval, exact payment, replay safety, and route eligibility before fulfillment.",
  },
  {
    icon: SearchCheck,
    title: "Discovery without fiction",
    description:
      "Discovery Mesh improves technical SEO and agent discovery. It does not guarantee search ranking.",
  },
] as const;

// TrustAndPayments explains where money moves and which safeguards protect fulfillment.
export function TrustAndPayments() {
  return (
    <section id="security" className="trust-surface border-y border-border py-[var(--section-space)]">
      <div className="site-container grid gap-12 lg:grid-cols-[0.8fr_1.2fr] lg:gap-20">
        <div>
          <p className="section-kicker text-primary-foreground/70">Trust Gate</p>
          <h2 className="mt-4 max-w-[12ch] font-display text-[clamp(2.5rem,5.5vw,5rem)] font-semibold leading-[0.98] tracking-[-0.06em] text-primary-foreground">
            Money moves to you. Proof stays visible.
          </h2>
          <p className="mt-6 max-w-lg text-base leading-7 text-primary-foreground/65">
            AgentPay is the verification and commerce layer—not a custodian. Every channel follows
            the same frozen price, evidence, and exactly-once fulfillment rules.
          </p>
        </div>
        <div className="grid gap-px overflow-hidden rounded-[1.75rem] border border-white/12 bg-white/12 sm:grid-cols-2">
          {trustPoints.map((point) => (
            <article key={point.title} className="bg-foreground p-6 sm:p-7">
              <point.icon className="size-5 text-emerald-300" aria-hidden="true" />
              <h3 className="mt-8 font-display text-xl font-semibold tracking-[-0.035em] text-primary-foreground">
                {point.title}
              </h3>
              <p className="mt-3 text-sm leading-6 text-primary-foreground/60">{point.description}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
