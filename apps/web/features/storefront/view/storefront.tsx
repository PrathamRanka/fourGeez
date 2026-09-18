import {
  AlertTriangle,
  ArrowRight,
  BadgeCheck,
  Clock3,
  ShieldCheck,
} from "lucide-react";
import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { buttonVariants } from "@/components/ui/button";
import { CommerceCheckout } from "@/features/commerce/view/commerce-checkout";
import type {
  DiscoverySignature,
  PublicProduct,
  StorefrontAvailabilityState,
  StorefrontManifest,
} from "@/features/storefront/model";
import { storefrontProductPath } from "@/features/storefront/model";
import { formatAtomicPrice, formatAtomicUnits } from "@/lib/money";
import styles from "./storefront.module.css";

export function StorefrontHome({
  manifest,
  signature,
}: {
  manifest: StorefrontManifest;
  signature: DiscoverySignature;
}) {
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.rail}>
        <header className={styles.hero}>
          <AuthoritativeStatus
            expiresAt={manifest.expiresAt}
            publicationRevision={manifest.publicationRevision}
            signature={signature}
          />
          <h1>{manifest.seller.name}</h1>
          <p className={styles.heroCopy}>
            Purchase published digital products through AgentPay&apos;s shared,
            exact-price x402 commerce flow.
          </p>
          <div className={styles.heroMeta}>
            <code>/store/{manifest.seller.slug}</code>
            <Link href={`/store/${manifest.seller.slug}/manifest.json`}>
              Signed manifest
            </Link>
            <Link href={`/store/${manifest.seller.slug}/llms.txt`}>
              llms.txt
            </Link>
          </div>
        </header>
        <section className={styles.catalog} aria-labelledby="catalog-title">
          <div className={styles.catalogHeader}>
            <div>
              <p className={styles.kicker}>Available now</p>
              <h2 id="catalog-title">Verified catalog</h2>
            </div>
            <span className={styles.catalogCount}>
              {manifest.products.length.toString().padStart(2, "0")} published
            </span>
          </div>
          {manifest.products.length === 0 ? (
            <div className={styles.empty}>
              <h3>No products are published yet</h3>
              <p>Check back after this seller publishes a product.</p>
            </div>
          ) : (
            <div className={styles.productGrid}>
              {manifest.products.map((product, index) => (
                <article className={styles.productCard} key={product.routeId}>
                  <div className={styles.productTopline}>
                    <span>Published product</span>
                    <span>0{index + 1}</span>
                  </div>
                  <h3>{product.displayName}</h3>
                  <p>{product.description}</p>
                  <div className={styles.productFooter}>
                    <div>
                      <strong>
                        {formatAtomicPrice(product.amount, product.asset)}
                      </strong>
                      <small>
                        Output: {outputFormatLabel(product.mimeType)}
                      </small>
                    </div>
                    <Link
                      className={`${buttonVariants()} ${styles.action}`}
                      href={storefrontProductPath(
                        manifest.seller.slug,
                        product.productSlug,
                      )}
                    >
                      View product <ArrowRight aria-hidden="true" />
                    </Link>
                  </div>
                </article>
              ))}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}

export function ProductDetail({
  sellerSlug,
  product,
  signature,
}: {
  sellerSlug: string;
  product: PublicProduct;
  signature: DiscoverySignature;
}) {
  const productPath = storefrontProductPath(sellerSlug, product.productSlug);
  return (
    <main id="main-content" className={styles.page}>
      <div className={styles.rail}>
        <Link className={styles.back} href={`/store/${sellerSlug}`}>
          ← Back to storefront
        </Link>
        <div className={styles.productLayout}>
          <article className={styles.productInfo}>
            <p className={styles.kicker}>Active AgentPay product</p>
            <h1>{product.displayName}</h1>
            <span className={styles.productDescription}>
              {product.description}
            </span>
            <dl className={styles.facts}>
              <div>
                <dt>Exact price</dt>
                <dd>{formatAtomicPrice(product.amount, product.asset)}</dd>
              </div>
              <div>
                <dt>Output format</dt>
                <dd>{outputFormatLabel(product.mimeType)}</dd>
              </div>
              <div>
                <dt>Trust status</dt>
                <dd>Active on AgentPay</dd>
              </div>
            </dl>
            <div className={styles.url}>
              <span>Product URL</span>
              <code>{productPath}</code>
            </div>
            <Accordion className={styles.technical}>
              <AccordionItem value="technical-details">
                <AccordionTrigger>Advanced technical details</AccordionTrigger>
                <AccordionContent>
                  <dl>
                    <div>
                      <dt>Route ID</dt>
                      <dd>{product.routeId}</dd>
                    </div>
                    <div>
                      <dt>Output MIME type</dt>
                      <dd>{product.mimeType}</dd>
                    </div>
                    <div>
                      <dt>Payment network</dt>
                      <dd>{product.network}</dd>
                    </div>
                    <div>
                      <dt>Atomic amount</dt>
                      <dd>{product.amount}</dd>
                    </div>
                    <div>
                      <dt>Purchase session endpoint</dt>
                      <dd>{product.purchaseSessionEndpoint}</dd>
                    </div>
                    <div>
                      <dt>Discovery signing key</dt>
                      <dd>{signature.kid}</dd>
                    </div>
                  </dl>
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </article>
          <aside className={styles.checkoutAside}>
            <p className={styles.checkoutLabel}>Exact-price checkout</p>
            <CommerceCheckout
              channel="browser"
              product={product}
              sellerSlug={sellerSlug}
            />
          </aside>
        </div>
      </div>
    </main>
  );
}

export function StorefrontAvailability({
  state,
  subject = "storefront",
}: {
  state:
    | StorefrontAvailabilityState
    | {
        status: "inactive" | "expired" | "unavailable";
        sellerSlug: string;
        reason?: string;
        expiresAt?: string;
      };
  subject?: "storefront" | "product";
}) {
  const copy = availabilityCopy(state.status, subject);
  const Icon =
    state.status === "inactive"
      ? ShieldCheck
      : state.status === "expired"
        ? Clock3
        : AlertTriangle;
  return (
    <main id="main-content" className={styles.availability}>
      <section
        className={styles.availabilityCard}
        aria-labelledby="availability-title"
      >
        <div className={styles.availabilityMark}>
          <Icon aria-hidden="true" />
        </div>
        <p className={styles.kicker}>AgentPay availability</p>
        <h1 id="availability-title">{copy.heading}</h1>
        <span>{copy.description}</span>
        <div className={styles.availabilityActions}>
          {state.status === "unavailable" ? (
            <Link
              className={`${buttonVariants()} ${styles.action}`}
              href={`/store/${state.sellerSlug}`}
            >
              Try again
            </Link>
          ) : null}
          <Link
            className={`${buttonVariants({ variant: "outline" })} ${styles.action}`}
            href="/"
          >
            Return to AgentPay
          </Link>
        </div>
        <small>
          No purchase can start from this page until AgentPay reports an active,
          current product.
        </small>
      </section>
    </main>
  );
}

function AuthoritativeStatus({
  expiresAt,
  publicationRevision,
  signature,
}: {
  expiresAt: string;
  publicationRevision: number;
  signature: DiscoverySignature;
}) {
  return (
    <div className={styles.status} aria-label="AgentPay discovery status">
      <BadgeCheck aria-hidden="true" />
      <div>
        <strong>Active on AgentPay</strong>
        <span>
          Signed revision {publicationRevision} · current until{" "}
          <time dateTime={expiresAt}>{formatDiscoveryTime(expiresAt)}</time>
        </span>
      </div>
      <code>{signature.alg}</code>
    </div>
  );
}

function availabilityCopy(
  status: "inactive" | "expired" | "unavailable",
  subject: "storefront" | "product",
) {
  if (status === "inactive")
    return {
      heading: `This ${subject} is not accepting new purchases`,
      description:
        "AgentPay has disabled discovery and checkout. Previously copied links or seller-hosted metadata cannot reactivate it.",
    };
  if (status === "expired")
    return {
      heading: `This ${subject} status has expired`,
      description:
        "The last signed discovery document is no longer current, so AgentPay will not present it as purchasable.",
    };
  return {
    heading: `This ${subject} is temporarily unavailable`,
    description:
      "AgentPay could not confirm a current signed status. Try again before starting a purchase.",
  };
}

function formatDiscoveryTime(timestamp: string): string {
  return new Intl.DateTimeFormat("en", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(new Date(timestamp));
}

function outputFormatLabel(mimeType: string): string {
  const labels: Record<string, string> = {
    "application/json": "JSON",
    "text/csv": "CSV",
    "text/plain": "Plain text",
  };
  return labels[mimeType] ?? mimeType;
}

export function buildProductStructuredData(product: PublicProduct) {
  return {
    "@context": "https://schema.org",
    "@type": "Service",
    name: product.displayName,
    description: product.description,
    url: product.canonicalUrl,
    offers: {
      "@type": "Offer",
      price: formatAtomicUnits(product.amount),
      priceCurrency: product.asset,
      availability: "https://schema.org/InStock",
    },
  };
}
