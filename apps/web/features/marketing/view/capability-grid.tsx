import {
  Bot,
  ChartNoAxesCombined,
  Code2,
  Fingerprint,
  Radar,
  ShieldCheck,
} from "lucide-react";
import { productCapabilities } from "@/features/marketing/model";

const capabilityIcons = {
  agent: Bot,
  analytics: ChartNoAxesCombined,
  code: Code2,
  globe: Radar,
  proof: Fingerprint,
  security: ShieldCheck,
} as const;

// CapabilityGrid describes each branded AgentPay capability in plain language.
export function CapabilityGrid() {
  return (
    <section id="product" className="border-y border-border bg-card py-[var(--section-space)]">
      <div className="site-container">
        <div className="grid items-end gap-6 md:grid-cols-2">
          <div>
            <p className="section-kicker">One connected system</p>
            <h2 className="section-title mt-4 max-w-[13ch]">
              The commerce layer your API was missing.
            </h2>
          </div>
          <p className="section-copy max-w-xl md:justify-self-end">
            AgentPay connects discovery, payment, fulfillment, and evidence without asking you to
            rebuild the product customers already value.
          </p>
        </div>
        <div className="mt-12 grid overflow-hidden rounded-[2rem] border border-border md:grid-cols-2 lg:grid-cols-3">
          {productCapabilities.map((capability) => {
            const Icon = capabilityIcons[capability.icon];
            return (
              <article
                key={capability.title}
                className="capability-card min-h-64 border-b border-border p-6 last:border-b-0 md:border-r md:[&:nth-child(2n)]:border-r-0 lg:[&:nth-child(2n)]:border-r lg:[&:nth-child(3n)]:border-r-0"
              >
                <div className="grid size-10 place-items-center rounded-xl border border-primary/15 bg-primary/5 text-primary">
                  <Icon className="size-5" aria-hidden="true" />
                </div>
                <p className="mt-10 font-mono text-[0.6875rem] uppercase tracking-[0.13em] text-muted-foreground">
                  {capability.eyebrow}
                </p>
                <h3 className="mt-2 font-display text-2xl font-semibold tracking-[-0.045em]">
                  {capability.title}
                </h3>
                <p className="mt-3 text-sm leading-6 text-muted-foreground">
                  {capability.description}
                </p>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
