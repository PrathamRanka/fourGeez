import {
  ArrowRight,
  Check,
  CircleDollarSign,
  Cloud,
  Code2,
  FileCheck2,
  KeyRound,
  LockKeyhole,
  Radar,
  ReceiptText,
  ShieldCheck,
  Webhook,
} from "lucide-react";
import Link from "next/link";
import AnimatedGradientBackground from "@/components/ui/animated-gradient-background";
import { IntegrationCard } from "@/components/ui/integration-card";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";
import {
  agentPayPlans,
  launchSignals,
  supportedStacks,
} from "@/features/marketing/model";
import { CommerceDemo } from "@/features/marketing/view/commerce-demo";
import { FrequentlyAskedQuestions } from "@/features/marketing/view/frequently-asked-questions";
import { HeroMascot } from "@/features/marketing/view/hero-mascot";
import styles from "./marketing-page.module.css";

const setupSteps = [
  ["01", "Connect", "Add one project key to your coding agent."],
  ["02", "Review", "Approve routes, prices, and deployment changes."],
  ["03", "Publish", "Open a verified storefront to people and agents."],
] as const;

function ArrowLink({ href, children }: { href: string; children: string }) {
  return (
    <Link className={styles.arrowLink} href={href}>
      {children}
      <ArrowRight aria-hidden="true" />
    </Link>
  );
}

function HeroSection() {
  return (
    <section className={styles.hero} aria-labelledby="hero-title">
      <div className={styles.themeControl}>
        <ThemeCycleButton />
      </div>
      <AnimatedGradientBackground
        breathing
        animationSpeed={0.025}
        breathingRange={6}
        containerClassName={styles.heroGradient}
        startingGap={120}
        topOffset={8}
      />
      <div className={styles.heroCopy}>
        <HeroMascot className={styles.heroMascot} />
        <p className={styles.kicker}>Commerce infrastructure for software</p>
        <h1 id="hero-title">Sell to agents. Settle on-chain.</h1>
        <p className={styles.heroDescription}>
          The next visitor to your site will be an AI agent.
          Make them your next customer.
        </p>
        <div className={styles.heroActions}>
          <Link className={styles.primaryAction} href="/sign-up">
            Start selling
            <ArrowRight aria-hidden="true" />
          </Link>
          <Link className={styles.secondaryAction} href="/demo/agent-checkout">
            Watch agent checkout
          </Link>
        </div>
      </div>

      <div
        className={styles.heroRail}
        aria-label="Illustrative AgentPay purchase"
      >
        <div className={styles.railPrompt}>
          <span>Buyer agent</span>
          <strong>Buy the market brief under 0.10 USDC</strong>
        </div>
        <ol className={styles.railSteps}>
          <li>
            <Radar aria-hidden="true" />
            <span>Discover</span>
          </li>
          <li>
            <CircleDollarSign aria-hidden="true" />
            <span>Pay x402</span>
          </li>
          <li>
            <ShieldCheck aria-hidden="true" />
            <span>Verify</span>
          </li>
          <li>
            <FileCheck2 aria-hidden="true" />
            <span>Fulfill</span>
          </li>
        </ol>
        <div className={styles.railResult}>
          <span className={styles.liveDot} aria-hidden="true" />
          <span>200 · signed result</span>
          <code>842 ms</code>
        </div>
      </div>
    </section>
  );
}

function SignalStrip() {
  return (
    <section className={styles.signalStrip} aria-label="AgentPay product facts">
      <div className={styles.signalIntro}>
        Built for the stack you already run
      </div>
      {launchSignals.map((signal) => (
        <div className={styles.signal} key={signal.label}>
          <strong>{signal.value}</strong>
          <span>{signal.label}</span>
        </div>
      ))}
      <div className={styles.stackTicker} aria-label="Supported stacks">
        <div className={styles.stackTrack}>
          {[...supportedStacks, ...supportedStacks].map((stack, index) => (
            <span key={`${stack}-${index}`}>{stack}</span>
          ))}
        </div>
      </div>
    </section>
  );
}

function SetupTerminal() {
  return (
    <div className={styles.setupTerminal} aria-label="AgentPay setup sequence">
      <div className={styles.terminalBar}>
        <span />
        <span />
        <span />
        <code>agentpay / setup</code>
      </div>
      <p>
        <span>$</span> connect this repository to AgentPay
      </p>
      <ul>
        <li>
          <Check aria-hidden="true" /> Sellable routes mapped
        </li>
        <li>
          <Check aria-hidden="true" /> x402 gate prepared
        </li>
        <li>
          <Check aria-hidden="true" /> Discovery files generated
        </li>
        <li>
          <LockKeyhole aria-hidden="true" /> Waiting for seller approval
        </li>
      </ul>
    </div>
  );
}

function RevenuePreview() {
  const points =
    "4,86 42,78 78,80 116,55 152,63 190,42 226,48 264,20 300,27 338,10";

  return (
    <div
      className={styles.revenuePreview}
      aria-label="Illustrative asset-separated revenue"
    >
      <div className={styles.revenueHeader}>
        <div>
          <span>Verified volume</span>
          <strong>84.20 USDC</strong>
        </div>
        <span>Base Sepolia</span>
      </div>
      <svg
        viewBox="0 0 342 96"
        role="img"
        aria-label="Illustrative upward verified payment activity"
      >
        <defs>
          <linearGradient id="marketing-chart-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="currentColor" stopOpacity="0.35" />
            <stop offset="1" stopColor="currentColor" stopOpacity="0" />
          </linearGradient>
        </defs>
        <path
          d={`M ${points} L 338 96 L 4 96 Z`}
          fill="url(#marketing-chart-fill)"
        />
        <polyline
          points={points}
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className={styles.revenueRows}>
        <span>
          Fulfilled <strong>1,024</strong>
        </span>
        <span>
          Disputed <strong>0</strong>
        </span>
      </div>
    </div>
  );
}

function ProductBento() {
  return (
    <section
      id="product"
      className={styles.section}
      aria-labelledby="product-title"
    >
      <header className={styles.sectionHeading}>
        <p className={styles.kicker}>The product</p>
        <h2 id="product-title">Your route to revenue.</h2>
        <p>From repository to paid response. One controlled path.</p>
      </header>

      <div className={styles.bento}>
        <article className={`${styles.bentoCard} ${styles.launchCard}`}>
          <div className={styles.cardCopy}>
            <span className={styles.cardIcon}>
              <Code2 aria-hidden="true" />
            </span>
            <p className={styles.cardLabel}>Launch Rail</p>
            <h3>Launch with one prompt</h3>
            <p>
              Your coding agent prepares the integration. You approve the
              production change.
            </p>
          </div>
          <SetupTerminal />
        </article>

        <article className={`${styles.bentoCard} ${styles.checkoutCard}`}>
          <div className={styles.cardCopy}>
            <span className={styles.cardIcon}>
              <CircleDollarSign aria-hidden="true" />
            </span>
            <p className={styles.cardLabel}>Agent Checkout</p>
            <h3>Agents pay. Your API responds.</h3>
            <p>
              The seller quote is fixed. Payment clears before one signed
              fulfillment call.
            </p>
          </div>
          <CommerceDemo />
        </article>

        <article className={`${styles.bentoCard} ${styles.revenueCard}`}>
          <div className={styles.cardCopy}>
            <span className={styles.cardIcon}>
              <ReceiptText aria-hidden="true" />
            </span>
            <p className={styles.cardLabel}>Revenue Lens</p>
            <h3>Track every verified sale</h3>
            <p>
              Payments, fulfillment, evidence, and disputes stay reconciled by
              asset and network.
            </p>
          </div>
          <RevenuePreview />
        </article>

        <article
          id="security"
          className={`${styles.bentoCard} ${styles.controlCard}`}
        >
          <div className={styles.cardCopy}>
            <span className={styles.cardIcon}>
              <Cloud aria-hidden="true" />
            </span>
            <p className={styles.cardLabel}>Trust Gate</p>
            <h3>Cloud authority. Local freedom.</h3>
            <p>
              Your connector can run locally. AgentPay still controls live
              access, publication, and transaction authority.
            </p>
          </div>
          <div
            className={styles.controlLayers}
            aria-label="AgentPay authorization boundaries"
          >
            <div>
              <KeyRound aria-hidden="true" />
              <span>Project key</span>
              <small>Bootstrap only</small>
            </div>
            <div>
              <ShieldCheck aria-hidden="true" />
              <span>Capability</span>
              <small>Short-lived</small>
            </div>
            <div>
              <Webhook aria-hidden="true" />
              <span>Execution</span>
              <small>Signed once</small>
            </div>
          </div>
        </article>
      </div>
    </section>
  );
}

function IntegrationSection() {
  return (
    <section
      id="how-it-works"
      className={styles.integrationSection}
      aria-labelledby="integration-title"
    >
      <div className={styles.integrationCopy}>
        <p className={styles.kicker}>Integration</p>
        <h2 id="integration-title">One integration. Every sale.</h2>
        <p>
          AgentPay sits between discovery and fulfillment without taking custody
          of buyer funds.
        </p>
        <ol className={styles.setupSteps}>
          {setupSteps.map(([number, title, description]) => (
            <li key={number}>
              <span>{number}</span>
              <div>
                <strong>{title}</strong>
                <p>{description}</p>
              </div>
            </li>
          ))}
        </ol>
        <ArrowLink href="/docs">Read the integration guide</ArrowLink>
      </div>

      <div className={styles.integrationCardFrame}>
        <IntegrationCard />
      </div>
    </section>
  );
}

function PricingSection() {
  return (
    <section
      id="pricing"
      className={styles.pricingSection}
      aria-label="AgentPay plans"
    >
      <header className={styles.centeredHeading}>
        <p className={styles.kicker}>Plans</p>
        <h2>Scale when your catalog does.</h2>
        <p>
          Implemented limits, clearly separated. Launch pricing is configured
          through seller billing.
        </p>
      </header>
      <div className={styles.pricingGrid}>
        {agentPayPlans.map((plan) => (
          <article
            className={`${styles.planCard} ${plan.featured ? styles.featuredPlan : ""}`}
            key={plan.name}
          >
            {plan.featured ? (
              <span className={styles.popular}>Most capable for launch</span>
            ) : null}
            <div className={styles.planTop}>
              <h3>{plan.name}</h3>
              <p>{plan.audience}</p>
              <strong>{plan.volume}</strong>
              <span>{plan.volumeLabel}</span>
            </div>
            <Link
              className={plan.featured ? styles.planPrimary : styles.planAction}
              href="/sign-up"
            >
              Choose {plan.name}
            </Link>
            <ul>
              {plan.features.map((feature) => (
                <li key={feature}>
                  <Check aria-hidden="true" />
                  {feature}
                </li>
              ))}
            </ul>
          </article>
        ))}
      </div>
      <p className={styles.pricingNote}>
        Buyer funds settle directly to your verified wallet. AgentPay
        subscription billing is separate.
      </p>
    </section>
  );
}

function ClosingSection() {
  return (
    <section className={styles.closingSection}>
      <div className={styles.closingGlow} aria-hidden="true" />
      <p className={styles.kicker}>Your API is already valuable</p>
      <h2>Make it buyable.</h2>
      <p>Connect once. Approve what ships. Reconcile every sale.</p>
      <div className={styles.heroActions}>
        <Link className={styles.primaryAction} href="/sign-up">
          Create your storefront
          <ArrowRight aria-hidden="true" />
        </Link>
        <Link className={styles.secondaryAction} href="/docs">
          Read the docs
        </Link>
      </div>
    </section>
  );
}

// MarketingPage composes the public AgentPay story in a compact, reference-led grid.
export function MarketingPage() {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.frame}>
        <HeroSection />
        <SignalStrip />
        <ProductBento />
        <IntegrationSection />
        <PricingSection />
        <FrequentlyAskedQuestions />
        <ClosingSection />
      </div>
    </main>
  );
}
