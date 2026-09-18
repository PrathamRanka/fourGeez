import { Check, Code2, FileCheck2, Search, ShieldCheck } from "lucide-react";
import { CommerceDemo } from "@/features/marketing/view/commerce-demo";

const launchChecks = [
  "Products mapped",
  "x402 gate added",
  "SEO/AEO generated",
] as const;

// PromptPreview shows the setup request that sellers can hand to a connected coding agent.
function PromptPreview() {
  return (
    <div className="prompt-preview" aria-label="AgentPay setup prompt preview">
      <div className="prompt-window-bar" aria-hidden="true">
        <span />
        <span />
        <span />
      </div>
      <p className="prompt-command">
        Connect this repo to AgentPay. Find sellable routes, add verification,
        generate discovery, and prepare everything for my approval.
      </p>
      <ul className="prompt-checks">
        {launchChecks.map((check) => (
          <li key={check}>
            <Check className="size-3.5" aria-hidden="true" />
            {check}
          </li>
        ))}
      </ul>
    </div>
  );
}

// AnalyticsPreview demonstrates asset-separated reporting without presenting live customer data.
function AnalyticsPreview() {
  return (
    <div
      className="analytics-preview"
      aria-label="Illustrative seller analytics"
    >
      <div className="analytics-topline">
        <div>
          <span>Verified volume</span>
          <strong>84.20 USDC</strong>
        </div>
        <span className="analytics-period">Last 30 days</span>
      </div>
      <div className="analytics-chart" aria-hidden="true">
        {[18, 26, 22, 38, 34, 52, 49, 66, 62, 78, 74, 92].map(
          (height, index) => (
            <span key={index} style={{ height: `${height}%` }} />
          ),
        )}
      </div>
      <div className="analytics-row">
        <span>research-summary</span>
        <strong>61.80 USDC</strong>
      </div>
      <div className="analytics-row">
        <span>dataset-export</span>
        <strong>22.40 USDC</strong>
      </div>
    </div>
  );
}

// ApprovalPreview communicates the seller-controlled checks before publication and delivery.
function ApprovalPreview() {
  return (
    <div
      className="approval-preview"
      aria-label="Seller approval and verification controls"
    >
      <div className="approval-sidebar">
        <ShieldCheck className="size-5" aria-hidden="true" />
        <span>Trust Gate</span>
      </div>
      <div className="approval-main">
        <div className="approval-header">
          <div>
            <span>Publication review</span>
            <strong>Ready for your approval</strong>
          </div>
          <span className="approval-badge">All checks passed</span>
        </div>
        <div className="approval-check-grid">
          <div>
            <Code2 aria-hidden="true" />
            <span>Request verification</span>
            <strong>Installed</strong>
          </div>
          <div>
            <Search aria-hidden="true" />
            <span>Discovery files</span>
            <strong>Validated</strong>
          </div>
          <div>
            <FileCheck2 aria-hidden="true" />
            <span>Sandbox purchase</span>
            <strong>Passed once</strong>
          </div>
        </div>
      </div>
    </div>
  );
}

// ProductShowcase presents AgentPay's four core seller outcomes in a reference-aligned card grid.
export function ProductShowcase() {
  return (
    <section id="product" className="product-section">
      <div className="site-container">
        <header className="product-heading">
          <h2>
            Everything you need to sell to agents.
            <br />
            All in one place.
          </h2>
          <p>From repository to verified revenue, with you in control.</p>
        </header>

        <div className="product-grid">
          <article className="product-card product-card-prompt">
            <PromptPreview />
            <div className="product-card-copy">
              <h3>Launch with one prompt</h3>
              <p>
                Your coding agent prepares routes, storefront pages, payment
                verification, and discovery files for review.
              </p>
            </div>
          </article>

          <article className="product-card product-card-commerce">
            <CommerceDemo />
            <div className="product-card-copy">
              <h3>Agents pay. Your API responds.</h3>
              <p>
                Exact-price payment and approval checks finish before AgentPay
                forwards one signed request to your service.
              </p>
            </div>
          </article>

          <article className="product-card product-card-analytics">
            <AnalyticsPreview />
            <div className="product-card-copy">
              <h3>Track every verified sale</h3>
              <p>
                Funds settle directly to your verified wallet. AgentPay records
                fulfilled sales, failures, disputes, routes, assets, and
                networks without mixing currencies.
              </p>
            </div>
          </article>

          <article id="security" className="product-card product-card-approval">
            <ApprovalPreview />
            <div className="product-card-copy">
              <h3>Built around approval, not surprises</h3>
              <p>
                Prices, publishing, wallets, and production changes remain
                seller-approved from setup through fulfillment. Discovery
                improves crawlability but does not guarantee search ranking.
              </p>
            </div>
          </article>
        </div>
      </div>
    </section>
  );
}
