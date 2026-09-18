import { Braces, CircleDot, SquareTerminal } from "lucide-react";
import { AutoScrollingClientCarousel } from "@/components/ui/auto-scrolling-client-carousel";
import { supportedAgents, supportedStacks } from "@/features/marketing/model";

const agentMarks = [SquareTerminal, Braces, CircleDot] as const;
const stackClients = supportedStacks.map((stack) => ({ name: stack }));

// PartnerStrip shows compatible coding agents and stacks without claiming commercial partnerships.
export function PartnerStrip() {
  return (
    <section className="partner-section" aria-labelledby="partner-title">
      <div className="site-container partner-grid">
        <p id="partner-title" className="partner-label">
          Works with the tools sellers already use
        </p>
        {supportedAgents.map((agent, index) => {
          const Icon = agentMarks[index];

          return (
            <div className="partner-cell" key={agent}>
              <Icon className="size-5" aria-hidden="true" />
              <span>{agent}</span>
            </div>
          );
        })}
      </div>
      <div className="site-container partner-marquee">
        <AutoScrollingClientCarousel clients={stackClients} />
      </div>
    </section>
  );
}
