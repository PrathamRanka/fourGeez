import { ArrowRight, Bot, ShieldCheck, WalletCards } from "lucide-react";
import Link from "next/link";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { buttonVariants } from "@/components/ui/button";
import type { PaidRoute } from "@/features/products/model";
import {
  paidRouteUrl,
  storefrontProductPath,
  type StorefrontManifest,
} from "@/features/storefront/model";
import { formatAtomicPrice, formatAtomicUnits } from "@/lib/money";

export function StorefrontHome({ manifest }: { manifest: StorefrontManifest }) {
  return (
    <main id="main-content" className="seller-storefront">
      <header className="storefront-hero">
        <p>Verified digital services</p>
        <h1>{manifest.seller.name}</h1>
        <span>
          Purchase published API-backed products through AgentPay’s shared x402
          commerce flow.
        </span>
        <code>/store/{manifest.seller.slug}</code>
        <nav aria-label="Agent discovery">
          <Link href={`/store/${manifest.seller.slug}/manifest.json`}>
            Manifest
          </Link>
          <Link href={`/store/${manifest.seller.slug}/llms.txt`}>llms.txt</Link>
        </nav>
      </header>
      <section className="storefront-products" aria-labelledby="products-title">
        <div>
          <p>Available now</p>
          <h2 id="products-title">Digital products</h2>
        </div>
        <div className="storefront-product-grid">
          {manifest.routes.map((route) => (
            <article key={route.routeId}>
              <span>Published product</span>
              <h3>{route.displayName}</h3>
              <p>{route.description}</p>
              <div>
                <strong>{formatAtomicPrice(route.amount, route.asset)}</strong>
                <small>Output: {outputFormatLabel(route.mimeType)}</small>
              </div>
              <Link
                className={buttonVariants()}
                href={storefrontProductPath(
                  manifest.seller.slug,
                  route.productSlug,
                )}
              >
                View product <ArrowRight aria-hidden="true" />
              </Link>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}

export function ProductDetail({
  manifest,
  route,
  apiOrigin,
}: {
  manifest: StorefrontManifest;
  route: PaidRoute;
  apiOrigin: string;
}) {
  const paidUrl = paidRouteUrl(
    apiOrigin,
    manifest.seller.slug,
    route.pathPattern,
  );
  const productPath = storefrontProductPath(
    manifest.seller.slug,
    route.productSlug,
  );
  return (
    <main id="main-content" className="storefront-product-detail">
      <Link href={`/store/${manifest.seller.slug}`}>
        ← {manifest.seller.name}
      </Link>
      <div className="storefront-product-layout">
        <article>
          <p>Digital product</p>
          <h1>{route.displayName}</h1>
          <span>{route.description}</span>
          <dl>
            <div>
              <dt>Exact price</dt>
              <dd>{formatAtomicPrice(route.amount, route.asset)}</dd>
            </div>
            <div>
              <dt>Output format</dt>
              <dd>{outputFormatLabel(route.mimeType)}</dd>
            </div>
            <div>
              <dt>Trust status</dt>
              <dd>Payment verified before delivery</dd>
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
                    <dd>{route.routeId}</dd>
                  </div>
                  <div>
                    <dt>API path</dt>
                    <dd>{route.pathPattern}</dd>
                  </div>
                  <div>
                    <dt>HTTP method</dt>
                    <dd>{route.method}</dd>
                  </div>
                  <div>
                    <dt>Output MIME type</dt>
                    <dd>{route.mimeType}</dd>
                  </div>
                  <div>
                    <dt>Payment network</dt>
                    <dd>{route.network}</dd>
                  </div>
                  <div>
                    <dt>Service timeout</dt>
                    <dd>{route.upstreamTimeoutSeconds} seconds</dd>
                  </div>
                  <div>
                    <dt>Atomic amount</dt>
                    <dd>{route.amount}</dd>
                  </div>
                  <div>
                    <dt>Paid API URL</dt>
                    <dd>{paidUrl}</dd>
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
            Pay exactly {formatAtomicPrice(route.amount, route.asset)}
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
            <Bot aria-hidden="true" /> Agents can discover the same product
            through the storefront manifest.
          </div>
        </aside>
      </div>
    </main>
  );
}

function outputFormatLabel(mimeType: string): string {
  const labels: Record<string, string> = {
    "application/json": "JSON",
    "text/csv": "CSV",
    "text/plain": "Plain text",
  };
  return labels[mimeType] ?? mimeType;
}

export function buildProductStructuredData(
  manifest: StorefrontManifest,
  route: PaidRoute,
  canonicalUrl: string,
) {
  return {
    "@context": "https://schema.org",
    "@type": "Service",
    name: route.displayName,
    description: route.description,
    url: canonicalUrl,
    provider: { "@type": "Organization", name: manifest.seller.name },
    offers: {
      "@type": "Offer",
      price: formatAtomicUnits(route.amount),
      priceCurrency: route.asset,
      availability: "https://schema.org/InStock",
    },
  };
}
