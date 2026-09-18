import { productFacts } from "@/features/marketing/model";

// ManifestoSection establishes the market shift and AgentPay's concrete product facts.
export function ManifestoSection() {
  return (
    <section id="how-it-works" className="manifesto-section">
      <div className="site-container manifesto-heading-wrap">
        <h2 className="manifesto-heading">
          Agent commerce is here.
          <br />
          Is your API ready?
        </h2>
      </div>

      <dl className="site-container fact-grid">
        {productFacts.map((fact) => (
          <div className="fact-cell" key={fact.value}>
            <dt>{fact.value}</dt>
            <dd>{fact.label}</dd>
          </div>
        ))}
      </dl>

      <div className="site-container dotted-statement">
        <p>
          AgentPay turns an existing API into a seller-approved storefront with
          machine-readable discovery, exact-price payment verification, and one
          auditable fulfillment path.
        </p>
      </div>
    </section>
  );
}
