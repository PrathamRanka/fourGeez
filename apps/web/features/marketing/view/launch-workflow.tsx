import { launchSteps } from "@/features/marketing/model";

// LaunchWorkflow explains the seller setup as four explicit approval-aware steps.
export function LaunchWorkflow() {
  return (
    <section
      id="how-it-works"
      aria-label="How AgentPay launches a storefront"
      className="site-container py-[var(--section-space)]"
    >
      <div className="grid gap-10 lg:grid-cols-[0.72fr_1.28fr] lg:gap-16">
        <div>
          <p className="section-kicker">Launch Rail</p>
          <h2 className="section-title mt-4 max-w-[12ch]">From repository to revenue route.</h2>
          <p className="section-copy mt-5 max-w-md">
            Your coding agent does the integration work. You retain control of every route, price,
            wallet, and production change.
          </p>
        </div>
        <ol className="workflow-list border-t border-foreground/15">
          {launchSteps.map((step) => (
            <li
              key={step.number}
              className="grid gap-3 border-b border-foreground/15 py-6 sm:grid-cols-[4rem_10rem_1fr] sm:items-baseline"
            >
              <span className="font-mono text-xs text-primary">{step.number}</span>
              <h3 className="font-display text-xl font-semibold tracking-[-0.035em]">
                {step.title}
              </h3>
              <p className="max-w-xl text-sm leading-6 text-muted-foreground">{step.description}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
