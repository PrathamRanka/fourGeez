import { ArrowRight, Bot, ShieldCheck, WalletCards } from "lucide-react";
import Link from "next/link";
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
        <nav aria-label="Agent discovery">
          <Link href={`/store/${manifest.seller.slug}/manifest.json`}>Manifest</Link>
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
              <span>{route.method}</span>
              <h3>{route.pathPattern}</h3>
              <p>{route.description}</p>
              <div>
                <strong>{formatAtomicPrice(route.amount, route.asset)}</strong>
                <small>{route.network}</small>
              </div>
              <Link
                className={buttonVariants()}
                href={storefrontProductPath(manifest.seller.slug, route.productSlug)}
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
  const paidUrl = paidRouteUrl(apiOrigin, manifest.seller.slug, route.pathPattern);
  return (
    <main id="main-content" className="storefront-product-detail">
      <Link href={`/store/${manifest.seller.slug}`}>← {manifest.seller.name}</Link>
      <div className="storefront-product-layout">
        <article>
          <p>{route.method} · {route.mimeType}</p>
          <h1>{route.pathPattern}</h1>
          <span>{route.description}</span>
          <dl>
            <div><dt>Exact price</dt><dd>{formatAtomicPrice(route.amount, route.asset)}</dd></div>
            <div><dt>Network</dt><dd>{route.network}</dd></div>
            <div><dt>Response</dt><dd>{route.mimeType}</dd></div>
          </dl>
        </article>
        <aside aria-labelledby="checkout-title">
          <WalletCards aria-hidden="true" />
          <p>Agent Checkout</p>
          <h2 id="checkout-title">Pay exactly {formatAtomicPrice(route.amount, route.asset)}</h2>
          <ol>
            <li>Create an immutable purchase intent for this route.</li>
            <li>Complete approval if the returned policy requires it.</li>
            <li>Request the paid URL and satisfy its x402 payment challenge.</li>
          </ol>
          <code>{paidUrl}</code>
          <div><ShieldCheck aria-hidden="true" /> Payment and evidence are verified before fulfillment.</div>
          <div><Bot aria-hidden="true" /> Agents can discover the same offer through the manifest.</div>
        </aside>
      </div>
    </main>
  );
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
