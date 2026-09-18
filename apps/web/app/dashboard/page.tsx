import type { Metadata } from "next";
import { ArrowUpRight, Boxes, FileCheck2, Settings2 } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { loadAnalyticsSnapshot } from "@/features/analytics/controller";
import { buildPaymentPairSummaries } from "@/features/analytics/model";
import { getSellerSession } from "@/features/auth/server/session";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./overview.module.css";

export const metadata: Metadata = { title: "Seller dashboard" };

const operatingLinks = [
  {
    href: "/dashboard/products",
    label: "Manage products",
    description: "Price, verify, and publish what agents can buy.",
    icon: Boxes,
    index: "01",
  },
  {
    href: "/dashboard/transactions",
    label: "Review transactions",
    description: "Follow settlement, fulfillment, evidence, and disputes.",
    icon: FileCheck2,
    index: "02",
  },
  {
    href: "/dashboard/onboarding",
    label: "Storefront settings",
    description: "Maintain payment destinations and MCP deployment.",
    icon: Settings2,
    index: "03",
  },
] as const;

export default async function DashboardPage() {
  const session = await getSellerSession();
  if (!session) redirect("/sign-in?returnTo=%2Fdashboard");
  if (!session.principal.sellerId || !session.principal.onboardingComplete) {
    redirect("/dashboard/onboarding");
  }

  const snapshot = await loadAnalyticsSnapshot();
  const paymentPairs = buildPaymentPairSummaries(snapshot.aggregates);

  return (
    <div className={styles.workspace}>
      <header className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Seller network / live operations</p>
          <h1>Commerce command center</h1>
          <div className={styles.intro}>
            <h2 className={styles.sellerName}>{session.principal.name}</h2>
            <span>Control discovery, settlement, and delivery.</span>
          </div>
        </div>
        <div className={styles.networkStatus}>
          <span aria-hidden="true" />
          {snapshot.error ? "Reporting degraded" : "Network reporting online"}
        </div>
      </header>

      {snapshot.error ? (
        <div className={styles.errorState} role="alert">
          <div>
            <strong>Live reporting is unavailable</strong>
            <p>{snapshot.error}</p>
          </div>
          <Link href="/dashboard">Retry overview</Link>
        </div>
      ) : null}

      <section className={styles.pulsePanel} aria-labelledby="commerce-pulse">
        <div className={styles.pulseHeading}>
          <div>
            <p className={styles.sectionIndex}>01 / Commerce pulse</p>
            <h2 id="commerce-pulse">Every settlement pair. Kept separate.</h2>
          </div>
          <div
            className={styles.transactionCount}
            role="status"
            aria-label={`${snapshot.transactionCount} ${
              snapshot.transactionCount === 1 ? "transaction" : "transactions"
            }`}
          >
            <strong>{snapshot.transactionCount}</strong>
            <span>
              {snapshot.transactionCount === 1 ? "transaction" : "transactions"}
            </span>
          </div>
        </div>

        {paymentPairs.length > 0 ? (
          <div className={styles.pairGrid}>
            {paymentPairs.map((paymentPair, index) => (
              <section
                key={`${paymentPair.asset}:${paymentPair.network}`}
                className={styles.pairCard}
                data-accent={index === 0 ? "true" : undefined}
                aria-label={`${paymentPair.asset} on ${paymentPair.network}`}
              >
                <div className={styles.pairIdentity}>
                  <span>{paymentPair.network}</span>
                  <strong>{paymentPair.asset}</strong>
                </div>
                <dl>
                  <div>
                    <dt>Gross verified</dt>
                    <dd>
                      {formatAtomicPrice(
                        paymentPair.grossVerifiedAmount,
                        paymentPair.asset,
                      )}
                    </dd>
                  </div>
                  <div>
                    <dt>Fulfilled</dt>
                    <dd>
                      {formatAtomicPrice(
                        paymentPair.fulfilledAmount,
                        paymentPair.asset,
                      )}
                    </dd>
                  </div>
                </dl>
              </section>
            ))}
          </div>
        ) : (
          <div className={styles.emptyPulse}>
            <div>
              <strong>Waiting for the first settlement</strong>
              <p>Published products will report here after verified payment.</p>
            </div>
            <Link href="/dashboard/products">Review products</Link>
          </div>
        )}

        <div className={styles.pulseFooter}>
          <span>Unlike assets and networks are never combined.</span>
          <Link href="/dashboard/analytics" aria-label="Open analytics">
            Open analytics <ArrowUpRight aria-hidden="true" />
          </Link>
        </div>
      </section>

      <section
        aria-labelledby="seller-operations"
        className={styles.operations}
      >
        <div className={styles.sectionHeading}>
          <div>
            <p className={styles.sectionIndex}>02 / Operate</p>
            <h2 id="seller-operations">Move the storefront forward.</h2>
          </div>
        </div>
        <div className={styles.operationGrid}>
          {operatingLinks.map((operation) => {
            const Icon = operation.icon;
            return (
              <article key={operation.href} className={styles.operationCard}>
                <div className={styles.operationTopline}>
                  <span>{operation.index}</span>
                  <Icon aria-hidden="true" />
                </div>
                <div>
                  <h3>{operation.label}</h3>
                  <p>{operation.description}</p>
                </div>
                <Link href={operation.href} aria-label={operation.label}>
                  Open <ArrowUpRight aria-hidden="true" />
                </Link>
              </article>
            );
          })}
        </div>
      </section>
    </div>
  );
}
