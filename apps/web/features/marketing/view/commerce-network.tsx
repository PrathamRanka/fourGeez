import { supportedStacks } from "@/features/marketing/model";
import { CommerceDemo } from "@/features/marketing/view/commerce-demo";

// CommerceNetwork pairs the working transaction demo with supported integration stacks.
export function CommerceNetwork() {
  return (
    <section id="discovery" className="site-container py-[var(--section-space)]">
      <div className="mx-auto max-w-3xl text-center">
        <p className="section-kicker">Agent Checkout</p>
        <h2 className="section-title mt-4">One sale. Two sides. One verifiable history.</h2>
        <p className="section-copy mx-auto mt-5 max-w-2xl">
          Buyers see a clear payment challenge and receive the result. Sellers see the same verified
          transaction in Revenue Lens, separated by asset and network.
        </p>
      </div>
      <div className="mx-auto mt-12 max-w-5xl">
        <CommerceDemo />
      </div>
      <div className="mt-16 border-y border-border py-6">
        <p className="text-center font-mono text-[0.6875rem] uppercase tracking-[0.15em] text-muted-foreground">
          Stack-native setup for the tools you already use
        </p>
        <ul className="mt-5 flex flex-wrap justify-center gap-2" aria-label="Supported technology stacks">
          {supportedStacks.map((stack) => (
            <li
              key={stack}
              className="rounded-full border border-border bg-card px-3.5 py-2 text-sm font-medium shadow-sm"
            >
              {stack}
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
