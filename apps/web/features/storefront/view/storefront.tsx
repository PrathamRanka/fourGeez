import {
  AlertTriangle,
  ArrowRight,
  BadgeCheck,
  Bot,
  Clock3,
  ShieldCheck,
  WalletCards,
} from "lucide-react";
import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { buttonVariants } from "@/components/ui/button";
import type {
  DiscoverySignature,
  PublicProduct,
  StorefrontAvailabilityState,
  StorefrontManifest,
} from "@/features/storefront/model";
import { storefrontProductPath } from "@/features/storefront/model";
import { formatAtomicPrice, formatAtomicUnits } from "@/lib/money";

export function StorefrontHome({
  manifest,
  signature,
}: {
  manifest: StorefrontManifest;
  signature: DiscoverySignature;
}) {
  return (
    <main id="main-content" className="seller-storefront">
      <header className="storefront-hero">
        <AuthoritativeStatus
          expiresAt={manifest.expiresAt}
          publicationRevision={manifest.publicationRevision}
          signature={signature}
        />
        <h1>{manifest.seller.name}</h1>
        <span>
          Purchase published API-backed products through AgentPay’s shared x402
          commerce flow.
        </span>
        <code>/store/{manifest.seller.slug}</code>
        <nav aria-label="Agent discovery">
          <Link href={`/store/${manifest.seller.slug}/manifest.json`}>
            Signed manifest
          </Link>
          <Link href={`/store/${manifest.seller.slug}/llms.txt`}>llms.txt</Link>
        </nav>
      </header>
      <section className="storefront-products" aria-labelledby="products-title">
        <div>
          <p>Available now</p>
          <h2 id="products-title">Digital products</h2>
        </div>
        {manifest.products.length === 0 ? (
          <div className="storefront-empty-state">
            <h3>No products are published yet</h3>
            <p>Check back after this seller publishes a product.</p>
          </div>
        ) : (
          <div className="storefront-product-grid">
            {manifest.products.map((product) => (
              <article key={product.routeId}>
                <span>Published product</span>
                <h3>{product.displayName}</h3>
                <p>{product.description}</p>
                <div>
                  <strong>
                    {formatAtomicPrice(product.amount, product.asset)}
                  </strong>
                  <small>Output: {outputFormatLabel(product.mimeType)}</small>
                </div>
                <Link
                  className={buttonVariants()}
                  href={storefrontProductPath(
                    manifest.seller.slug,
                    product.productSlug,
                  )}
                >
                  View product <ArrowRight aria-hidden="true" />
                </Link>
              </article>
            ))}
          </div>
        )}
      </section>
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
    <main id="main-content" className="storefront-product-detail">
      <Link href={`/store/${sellerSlug}`}>← Back to storefront</Link>
      <div className="storefront-product-layout">
        <article>
          <p>Active AgentPay product</p>
          <h1>{product.displayName}</h1>
          <span>{product.description}</span>
          <dl>
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
          <div className="storefront-product-url">
            <span>Product URL</span>
            <code>{productPath}</code>
          </div>
          <Accordion className="storefront-technical-details">
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
        <aside aria-labelledby="checkout-title">
          <WalletCards aria-hidden="true" />
          <p>Agent Checkout</p>
          <h2 id="checkout-title">
            Pay exactly {formatAtomicPrice(product.amount, product.asset)}
          </h2>
          <ol>
            <li>Create a fixed purchase request for this product.</li>
            <li>Complete approval if the purchase requires it.</li>
            <li>Pay the exact amount before AgentPay delivers the result.</li>
          </ol>
          <div>
            <ShieldCheck aria-hidden="true" /> Payment and delivery proof are
            verified before fulfillment.
          </div>
          <div>
            <Bot aria-hidden="true" /> Discovery describes this product;
            AgentPay rechecks availability before every purchase.
          </div>
        </aside>
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
    <main id="main-content" className="storefront-availability">
      <section aria-labelledby="availability-title">
        <div className={`storefront-availability-mark ${state.status}`}>
          <Icon aria-hidden="true" />
        </div>
        <p>AgentPay availability</p>
        <h1 id="availability-title">{copy.heading}</h1>
        <span>{copy.description}</span>
        <div className="storefront-availability-actions">
          {state.status === "unavailable" ? (
            <Link
              className={buttonVariants()}
              href={`/store/${state.sellerSlug}`}
            >
              Try again
            </Link>
          ) : null}
          <Link className={buttonVariants({ variant: "outline" })} href="/">
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
    <div
      className="storefront-authority"
      aria-label="AgentPay discovery status"
    >
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
  if (status === "inactive") {
    return {
      heading: `This ${subject} is not accepting new purchases`,
      description:
        "AgentPay has disabled discovery and checkout. Previously copied links or seller-hosted metadata cannot reactivate it.",
    };
  }
  if (status === "expired") {
    return {
      heading: `This ${subject} status has expired`,
      description:
        "The last signed discovery document is no longer current, so AgentPay will not present it as purchasable.",
    };
  }
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
