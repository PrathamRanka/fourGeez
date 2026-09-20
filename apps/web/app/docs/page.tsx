import type { Metadata } from "next";
import Link from "next/link";
import { ArrowUpRight, Check, CircleAlert, Terminal } from "lucide-react";
import { sellerDocsMetadata } from "@/features/marketing/seo";
import { CopyCodeButton } from "./copy-code-button";
import { DocsNavigation, type DocsNavGroup } from "./docs-navigation";
import styles from "./docs.module.css";

export const metadata: Metadata = sellerDocsMetadata;

const navigationGroups: DocsNavGroup[] = [
  {
    label: "Get started",
    items: [
      { id: "quickstart", label: "Quickstart" },
      { id: "prerequisites", label: "Prerequisites", nested: true },
      { id: "project-key", label: "Project key", nested: true },
    ],
  },
  {
    label: "Core workflow",
    items: [
      { id: "connect-mcp", label: "Connect MCP" },
      { id: "integration-prompt", label: "Generate changes", nested: true },
      { id: "review-publish", label: "Review and publish", nested: true },
      { id: "commerce-flow", label: "Checkout path" },
      {
        id: "payment-capabilities",
        label: "Payment compatibility",
        nested: true,
      },
    ],
  },
  {
    label: "Reference",
    items: [
      { id: "supported-stacks", label: "Supported stacks" },
      { id: "security-model", label: "Security model" },
      { id: "current-limits", label: "Current limits" },
    ],
  },
];

const powerShellSetup = `$ConnectorEntry = Join-Path $env:LOCALAPPDATA "AgentPay\\mcp-connector\\0.1.0\\node_modules\\@agentpay\\local-mcp-connector\\dist\\cli.js"
if (-not (Test-Path -LiteralPath $ConnectorEntry)) { throw "Install the verified release artifact first." }
$env:AGENTPAY_API_BASE_URL = "https://api.example.agentpay"
$env:AGENTPAY_PROJECT_KEY = Read-Host "Paste the project key shown once" -MaskInput
node $ConnectorEntry --check
codex`;

const codexConfig = `[mcp_servers.agentpay]
command = "node"
args = ["C:\\Users\\SELLER\\AppData\\Local\\AgentPay\\mcp-connector\\0.1.0\\node_modules\\@agentpay\\local-mcp-connector\\dist\\cli.js"]
env_vars = ["AGENTPAY_API_BASE_URL", "AGENTPAY_PROJECT_KEY"]
required = true`;

const setupPrompt = `Connect this project to AgentPay. Inspect only bounded committed manifests and OpenAPI, detect one maintained stack, propose sellable routes plus truthful SEO/AEO changes, install AgentPay request verification, and generate focused tests. Ask me for every exact price and payout destination. Never invent or change prices or payout addresses, publish, rotate credentials, or deploy without my explicit confirmation.`;

const supportedStacks = [
  "Next.js",
  "React/Vite + Node API",
  "Remix",
  "Nuxt",
  "SvelteKit",
  "Astro",
  "Express",
  "Fastify",
  "NestJS",
  "Go net/http",
  "Gin",
  "Echo",
  "Fiber",
  "FastAPI",
  "Starlette",
  "Flask",
  "Django",
  "ASP.NET Core",
  "Spring Boot",
  "Rails",
  "Laravel",
] as const;

function CodeBlock({
  code,
  label,
  language,
}: {
  code: string;
  label: string;
  language: string;
}) {
  const tokenPattern =
    /(#[^\n]*|"[^"]*"|'[^']*'|\$env:[A-Z_]+|@[A-Za-z0-9/_.-]+|--[A-Za-z-]+|\b(?:true|false|required|command|args|env_vars)\b)/g;

  function tokenClassName(token: string): string | undefined {
    if (token.startsWith("#")) return styles.syntaxComment;
    if (token.startsWith('"') || token.startsWith("'"))
      return styles.syntaxString;
    if (token.startsWith("$")) return styles.syntaxVariable;
    if (token.startsWith("@")) return styles.syntaxPackage;
    if (token.startsWith("--")) return styles.syntaxFlag;
    if (
      ["true", "false", "required", "command", "args", "env_vars"].includes(
        token,
      )
    ) {
      return styles.syntaxKeyword;
    }
    return undefined;
  }

  return (
    <div className={styles.codeBlock}>
      <div className={styles.codeHeader}>
        <span>{language}</span>
        <CopyCodeButton code={code} label={label} />
      </div>
      <pre tabIndex={0} aria-label={`${label} code example`}>
        <code>
          {code.split("\n").map((line, lineIndex) => (
            <span className={styles.codeLine} key={`${lineIndex}-${line}`}>
              {line.split(tokenPattern).map((token, tokenIndex) => {
                const className = tokenClassName(token);
                return className ? (
                  <span className={className} key={`${tokenIndex}-${token}`}>
                    {token}
                  </span>
                ) : (
                  token
                );
              })}
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

export default function DocsPage() {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.mobileNav}>
        <DocsNavigation groups={navigationGroups} mobile />
      </div>

      <div className={styles.docsGrid}>
        <aside className={styles.sidebar}>
          <Link className={styles.docsBrand} href="/">
            <span className={styles.brandSignal} aria-hidden="true" />
            AgentPay docs
          </Link>
          <DocsNavigation groups={navigationGroups} />
          <div className={styles.sidebarStatus}>
            <span>API status</span>
            <strong>Development preview</strong>
          </div>
        </aside>

        <article className={styles.article}>
          <header className={styles.hero}>
            <p className={styles.eyebrow}>Seller integration guide</p>
            <h1>Build with AgentPay</h1>
            <p className={styles.lede}>
              Turn an existing HTTPS API into a reviewable storefront for people
              and software agents—without handing AgentPay your wallet keys or
              deployment authority.
            </p>
            <div className={styles.statusNote} role="note">
              <CircleAlert aria-hidden="true" />
              <div>
                <strong>Development preview</strong>
                <p>
                  The seller workflow is verified locally. Payments are limited
                  to mock mode or Base Sepolia USDC testnet while AWS deployment
                  and release verification remain in progress.
                </p>
              </div>
            </div>
          </header>

          <section className={styles.section} id="quickstart">
            <p className={styles.sectionLabel}>Get started</p>
            <h2>Quickstart</h2>
            <p>
              AgentPay uses a local connector as the only supported production-
              shaped MCP path. The connector exchanges your project key for a
              short-lived cloud capability; the key is never sent to
              <code>/mcp</code> directly.
            </p>
            <p>
              The coding agent edits your repository using AgentPay MCP analysis
              and guidance. AgentPay MCP does not write repository files; it
              provides bounded configuration and verification tools, and cloud
              mutations remain subject to seller confirmation.
            </p>

            <div className={styles.workflow} aria-label="Integration flow">
              <span>Your repository</span>
              <span aria-hidden="true">→</span>
              <span>Local connector</span>
              <span aria-hidden="true">→</span>
              <span>AgentPay cloud</span>
              <span aria-hidden="true">→</span>
              <span>Storefront</span>
            </div>

            <div className={styles.subsection} id="prerequisites">
              <h3>1. Complete the prerequisites</h3>
              <p>
                Before key creation appears, the seller must have a verified
                account and profile, an active launch entitlement, a verified
                supported testnet payout address, and a reachable HTTPS service
                origin.
              </p>
              <ul className={styles.checklist}>
                <li>
                  <Check aria-hidden="true" /> Verified seller identity
                </li>
                <li>
                  <Check aria-hidden="true" /> HTTPS fulfillment origin
                </li>
                <li>
                  <Check aria-hidden="true" /> Base Sepolia USDC destination
                </li>
                <li>
                  <Check aria-hidden="true" /> Active launch entitlement
                </li>
              </ul>
            </div>

            <div className={styles.subsection} id="project-key">
              <h3>2. Create a project key</h3>
              <p>
                Each <code>apc2</code> key contains a fresh 256-bit secret from
                a cryptographically secure random generator. AgentPay shows the
                raw key once and stores only an HMAC-SHA-256 digest protected by
                a cloud-held pepper. Rotation creates a new random successor and
                revokes the previous key.
              </p>
              <div className={styles.callout}>
                <strong>Keep it server-side.</strong>
                <span>
                  Store the key in your local secret environment. Never commit
                  it, expose it through <code>NEXT_PUBLIC_*</code>, or paste it
                  into client code.
                </span>
              </div>
            </div>
          </section>

          <section className={styles.section} id="connect-mcp">
            <p className={styles.sectionLabel}>Core workflow</p>
            <h2>Connect the MCP client</h2>
            <p>
              Configure Claude Code, Codex, or a generic MCP host to launch the
              pinned local connector over stdio. First download the exact
              versioned GitHub Release asset, verify its checksum and source
              provenance, and install it in the documented versioned local
              directory. This example uses Codex and a PowerShell session.
            </p>
            <CodeBlock
              code={codexConfig}
              label="Copy Codex MCP configuration"
              language=".codex/config.toml"
            />
            <CodeBlock
              code={powerShellSetup}
              label="Copy PowerShell setup"
              language="PowerShell"
            />
            <p className={styles.caption}>
              Replace the example API origin with the environment-specific
              origin shown in the seller dashboard and replace the example
              <code>SELLER</code> path with the installed connector path. Never
              substitute an npm registry command or mutable Git URL for the
              verified release artifact.
            </p>

            <div className={styles.subsection} id="integration-prompt">
              <h3>Generate reviewable changes</h3>
              <p>
                Start with a bounded prompt. The coding agent may inspect
                committed manifests and OpenAPI, identify an evidenced stack,
                install verification, and prepare tests. It cannot choose prices
                or payout addresses for you.
              </p>
              <CodeBlock
                code={setupPrompt}
                label="Copy AgentPay setup prompt"
                language="Prompt"
              />
            </div>

            <div className={styles.subsection} id="review-publish">
              <h3>Review, confirm, then publish</h3>
              <ol className={styles.numberedList}>
                <li>
                  <span>01</span>
                  <div>
                    <strong>Review the diff</strong>
                    <p>
                      Check routes, exact prices, verification middleware,
                      discovery files, and tests.
                    </p>
                  </div>
                </li>
                <li>
                  <span>02</span>
                  <div>
                    <strong>Confirm the exact mutation</strong>
                    <p>
                      The seller dashboard issues a one-time grant bound to the
                      reviewed operation and arguments.
                    </p>
                  </div>
                </li>
                <li>
                  <span>03</span>
                  <div>
                    <strong>Run sandbox validation</strong>
                    <p>
                      Verify payment gating, signatures, idempotency, metadata,
                      and fulfillment before publication.
                    </p>
                  </div>
                </li>
              </ol>
            </div>
          </section>

          <section className={styles.section} id="commerce-flow">
            <p className={styles.sectionLabel}>Runtime</p>
            <h2>Understand the checkout path</h2>
            <p>
              Browser wallets and software agents enter the same immutable
              purchase-intent pipeline. AgentPay verifies the exact quote and
              finalized testnet payment, claims fulfillment once, then forwards
              a short-lived signed execution request to the seller.
            </p>
            <div className={styles.subsection} id="payment-capabilities">
              <h3>Check payment compatibility</h3>
              <p>
                Read <code>GET /v1/payment-capabilities</code> before presenting
                payment. The development runtime currently enables exact x402
                on Base Sepolia USDC only. Browser and agent buyers use the same
                immutable quote and payment path; card checkout is not enabled.
              </p>
            </div>
            <div className={styles.sequence}>
              <div>
                <span>01</span>
                <strong>Discover</strong>
                <p>
                  Read hosted storefront metadata, signed manifests, or{" "}
                  <code>llms.txt</code>. Discovery is never authorization.
                </p>
              </div>
              <div>
                <span>02</span>
                <strong>Pay</strong>
                <p>
                  Authorize the exact Base Sepolia USDC quote. Underpayment and
                  overpayment are rejected.
                </p>
              </div>
              <div>
                <span>03</span>
                <strong>Fulfill</strong>
                <p>
                  AgentPay forwards one signed request only after final payment
                  and an atomic transaction claim.
                </p>
              </div>
              <div>
                <span>04</span>
                <strong>Verify</strong>
                <p>
                  Receipts, evidence, transaction state, and seller-reported
                  refund records remain in the control plane.
                </p>
              </div>
            </div>
          </section>

          <section className={styles.section} id="supported-stacks">
            <p className={styles.sectionLabel}>Reference</p>
            <h2>Maintained stacks</h2>
            <p>
              Stack detection must be backed by bounded repository evidence. The
              current maintained matrix contains these 21 recipes.
            </p>
            <ul className={styles.stackGrid}>
              {supportedStacks.map((stack) => (
                <li key={stack}>{stack}</li>
              ))}
            </ul>
          </section>

          <section className={styles.section} id="security-model">
            <h2>Security model</h2>
            <div className={styles.definitionGrid}>
              <div>
                <h3>AgentPay controls</h3>
                <p>
                  Identity, entitlement, key validation, publication, payment
                  verification, transaction state, signatures, receipts, and
                  evidence.
                </p>
              </div>
              <div>
                <h3>You control</h3>
                <p>
                  Your repository, service deployment, exact prices, payout
                  destination, wallet authorization, and upstream business
                  logic.
                </p>
              </div>
              <div>
                <h3>The agent can propose</h3>
                <p>
                  Repository changes, candidate routes, verification setup,
                  tests, and truthful SEO/AEO files.
                </p>
              </div>
              <div>
                <h3>The agent cannot approve</h3>
                <p>
                  Prices, payout changes, publication, credential rotation, or
                  production deployment.
                </p>
              </div>
            </div>
            <Link className={styles.inlineLink} href="/security">
              Read the security overview <ArrowUpRight aria-hidden="true" />
            </Link>
          </section>

          <section className={styles.section} id="current-limits">
            <h2>Current limits</h2>
            <ul className={styles.limitList}>
              <li>AgentPay is not yet a production-ready paid service.</li>
              <li>
                Payments are mock mode or Base Sepolia USDC x402 testnet only.
              </li>
              <li>
                Card checkout, mainnet, multiple assets, shipping, tax, and
                physical inventory are deferred.
              </li>
              <li>
                The global marketplace directory and ranking are deferred.
              </li>
              <li>
                Technical SEO and AEO improve discoverability but never
                guarantee ranking, traffic, or sales.
              </li>
            </ul>
            <div className={styles.nextStep}>
              <Terminal aria-hidden="true" />
              <div>
                <strong>Ready to prepare a seller project?</strong>
                <p>
                  Complete onboarding first; AgentPay reveals connector setup
                  only after every server-authoritative prerequisite passes.
                </p>
              </div>
              <Link href="/sign-up">Create account</Link>
            </div>
          </section>
        </article>

        <aside className={styles.toc}>
          <nav aria-label="On this page">
            <p>On this page</p>
            <a href="#quickstart">Quickstart</a>
            <a href="#connect-mcp">Connect MCP</a>
            <a href="#integration-prompt">Generate changes</a>
            <a href="#commerce-flow">Checkout path</a>
            <a href="#payment-capabilities">Payment compatibility</a>
            <a href="#supported-stacks">Supported stacks</a>
            <a href="#security-model">Security model</a>
            <a href="#current-limits">Current limits</a>
          </nav>
        </aside>
      </div>
    </main>
  );
}
