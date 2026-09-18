export type DiscoverySignature = {
  alg: "ES256";
  kid: string;
  canonicalization: "RFC8785";
  domainSeparator: "agentpay.discovery.v1";
  value: string;
};

export type PublicProduct = {
  sellerId: string;
  routeId: string;
  displayName: string;
  productSlug: string;
  description: string;
  mimeType: string;
  amount: string;
  asset: string;
  network: string;
  availability: "active";
  canonicalUrl: string;
  purchaseSessionEndpoint: string;
};

export type StorefrontManifest = {
  schemaVersion: "agentpay.discovery.v1";
  sellerId: string;
  seller: { name: string; slug: string };
  availability: "active";
  publicationRevision: number;
  issuedAt: string;
  expiresAt: string;
  canonicalOrigin: string;
  products: PublicProduct[];
};

export type PublicProductDocument = {
  schemaVersion: "agentpay.discovery.v1";
  sellerId: string;
  sellerSlug: string;
  publicationRevision: number;
  issuedAt: string;
  expiresAt: string;
  canonicalOrigin: string;
  product: PublicProduct;
};

export type StorefrontTombstone = {
  schemaVersion: "agentpay.discovery.v1";
  sellerId: string;
  sellerSlug: string;
  availability: "inactive";
  reason: "suspended" | "cancelled" | "closed";
  publicationRevision: number;
  issuedAt: string;
  expiresAt: string;
  canonicalOrigin: string;
};

export type SignedStorefrontManifest = {
  document: StorefrontManifest;
  signature: DiscoverySignature;
};

export type SignedStorefrontTombstone = {
  document: StorefrontTombstone;
  signature: DiscoverySignature;
};

export type SignedPublicProductDocument = {
  document: PublicProductDocument;
  signature: DiscoverySignature;
};

export type StorefrontDiscoveryState =
  | {
      status: "active";
      manifest: StorefrontManifest;
      signature: DiscoverySignature;
    }
  | {
      status: "inactive";
      tombstone: StorefrontTombstone;
      signature: DiscoverySignature;
    }
  | { status: "expired"; sellerSlug: string; expiresAt: string }
  | { status: "unavailable"; sellerSlug: string; reason: string };

export type PublicProductState =
  | ({ status: "active" } & SignedPublicProductDocument)
  | {
      status: "inactive";
      sellerSlug: string;
      reason: "suspended" | "cancelled" | "closed" | "seller_inactive";
    }
  | { status: "expired"; sellerSlug: string; expiresAt: string }
  | { status: "unavailable"; sellerSlug: string; reason: string };

export type StorefrontAvailabilityState = Exclude<
  StorefrontDiscoveryState,
  { status: "active" }
>;

export function storefrontProductPath(
  sellerSlug: string,
  productSlug: string,
): string {
  return `/store/${encodeURIComponent(sellerSlug)}/products/${encodeURIComponent(productSlug)}`;
}
