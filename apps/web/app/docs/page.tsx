import type { Metadata } from "next";
import { sellerDocsMetadata } from "@/features/marketing/seo";
import styles from "./docs.module.css";

export const metadata: Metadata = sellerDocsMetadata;

const integrationSteps = [
  [
    "project",
    "Create a project",
    "Create a seller storefront and issue one project-scoped integration key.",
  ],
  [
    "agent",
    "Connect your agent",
    "Add the AgentPay MCP server to Claude Code, Codex, or a generic MCP host.",
  ],
  [
    "integration",
    "Generate the integration",
    "Ask the coding agent to propose routes, prices, verification, discovery files, and tests.",
  ],
  [
    "verify",
    "Review and verify",
    "Approve the proposed changes, prove wallet control, and run the sandbox purchase.",
  ],
  [
    "publish",
    "Publish",
    "Publish only after AgentPay verifies signatures, payment gating, metadata, and fulfillment.",
  ],
] as const;

export default function DocsPage() {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.rail}>
        <aside className={styles.sidebar}>
          <p>Getting started</p>
          <nav aria-label="Documentation sections">
            {integrationSteps.map(([id, title]) => (
              <a href={`#${id}`} key={id}>
                {title}
              </a>
            ))}
            <a href="#payment-capabilities">Payment compatibility</a>
            <a href="#prompt">Setup prompt</a>
          </nav>
        </aside>
        <article className={styles.article}>
          <p className={styles.kicker}>Documentation / Seller setup</p>
          <h1>Connect AgentPay to your repository.</h1>
          <p className={styles.lede}>
            One scoped key. One MCP connection. One reviewable integration
            proposal.
          </p>
          <ol className={styles.steps}>
            {integrationSteps.map(([id, title, description], index) => (
              <li className={styles.step} id={id} key={id}>
                <span className={styles.number}>0{index + 1}</span>
                <div>
                  <h2>{title}</h2>
                  <p>{description}</p>
                </div>
              </li>
            ))}
          </ol>
          <section className={styles.prompt} id="payment-capabilities">
            <h2>Check payment compatibility</h2>
            <p>
              Read <code>GET /v1/payment-capabilities</code> before presenting
              payment. The testnet runtime currently enables exact x402 on Base
              Sepolia USDC only. Browser and agent buyers use the same immutable
              quote and payment path; card checkout is not enabled.
            </p>
          </section>
          <section className={styles.prompt} id="prompt">
            <h2>Representative setup prompt</h2>
            <p>
              Connect this project to AgentPay. Identify sellable API routes,
              propose products and prices, install request verification,
              generate the storefront and discovery metadata, run the tests, and
              prepare the changes for my approval.
            </p>
          </section>
        </article>
      </div>
    </main>
  );
}
