import type {
  DiscoverySignature,
  PublicProduct,
  PublicProductDocument,
  PublicProductState,
  SignedPublicProductDocument,
  SignedStorefrontManifest,
  SignedStorefrontTombstone,
  StorefrontDiscoveryState,
  StorefrontManifest,
  StorefrontTombstone,
} from "@/features/storefront/model";
import { storefrontProductPath } from "@/features/storefront/model";
import {
  requestPublicAgentPay,
  requestPublicAgentPayText,
  type ActionResult,
} from "@/lib/agentpay-api";

const discoverySchemaVersion = "agentpay.discovery.v1";
const productContractSchemaVersion = "agentpay.product-contract.v1";

export async function loadStorefrontDiscovery(
  sellerSlug: string,
): Promise<StorefrontDiscoveryState> {
  const result = await requestPublicAgentPay<
    SignedStorefrontManifest | SignedStorefrontTombstone
  >(`/store/${encodeURIComponent(sellerSlug)}/manifest.json`);

  if (
    result.ok &&
    isSignedStorefrontManifest(result.value) &&
    matchesStorefrontRequest(result.value.document, sellerSlug)
  ) {
    if (isExpired(result.value.document.expiresAt)) {
      return {
        status: "expired",
        sellerSlug,
        expiresAt: result.value.document.expiresAt,
      };
    }
    return {
      status: "active",
      manifest: result.value.document,
      signature: result.value.signature,
    };
  }

  if (
    !result.ok &&
    result.status === 410 &&
    isSignedStorefrontTombstone(result.responseBody) &&
    result.responseBody.document.sellerSlug === sellerSlug
  ) {
    if (isExpired(result.responseBody.document.expiresAt)) {
      return {
        status: "expired",
        sellerSlug,
        expiresAt: result.responseBody.document.expiresAt,
      };
    }
    return {
      status: "inactive",
      tombstone: result.responseBody.document,
      signature: result.responseBody.signature,
    };
  }

  return {
    status: "unavailable",
    sellerSlug,
    reason: result.ok
      ? "invalid_discovery_document"
      : (result.code ?? "not_found"),
  };
}

export async function loadPublicProduct(
  sellerSlug: string,
  productSlug: string,
): Promise<PublicProductState> {
  const result = await requestPublicAgentPay<SignedPublicProductDocument>(
    `/v1/storefronts/${encodeURIComponent(sellerSlug)}/products/${encodeURIComponent(productSlug)}`,
  );

  if (
    result.ok &&
    isSignedPublicProductDocument(result.value) &&
    matchesProductRequest(result.value.document, sellerSlug, productSlug)
  ) {
    if (isExpired(result.value.document.expiresAt)) {
      return {
        status: "expired",
        sellerSlug,
        expiresAt: result.value.document.expiresAt,
      };
    }
    return { status: "active", ...result.value };
  }

  if (!result.ok && result.status === 410) {
    return {
      status: "inactive",
      sellerSlug,
      reason: "seller_inactive",
    };
  }

  return {
    status: "unavailable",
    sellerSlug,
    reason: result.ok
      ? "invalid_discovery_document"
      : (result.code ?? "not_found"),
  };
}

export async function loadStorefrontLLMSText(
  slug: string,
): Promise<ActionResult<string>> {
  return requestPublicAgentPayText(
    `/store/${encodeURIComponent(slug)}/llms.txt`,
  );
}

function isExpired(expiresAt: string): boolean {
  const expiry = Date.parse(expiresAt);
  return !Number.isFinite(expiry) || expiry <= Date.now();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

function isPositiveAtomicAmount(value: unknown): value is string {
  return (
    typeof value === "string" &&
    /^\d+$/.test(value) &&
    BigInt(value) > BigInt(0)
  );
}

function isDiscoverySignature(
  value: unknown,
  domainSeparator: DiscoverySignature["domainSeparator"] = discoverySchemaVersion,
): value is DiscoverySignature {
  return (
    isRecord(value) &&
    value.alg === "ES256" &&
    isNonEmptyString(value.kid) &&
    value.canonicalization === "RFC8785" &&
    value.domainSeparator === domainSeparator &&
    isNonEmptyString(value.value)
  );
}

function isPublicProduct(value: unknown): value is PublicProduct {
  return (
    isRecord(value) &&
    value.schemaVersion === productContractSchemaVersion &&
    isNonEmptyString(value.sellerId) &&
    isNonEmptyString(value.routeId) &&
    Number.isInteger(value.routeVersion) &&
    isNonEmptyString(value.displayName) &&
    isNonEmptyString(value.productSlug) &&
    isNonEmptyString(value.description) &&
    isNonEmptyString(value.mimeType) &&
    isRecord(value.inputSchema) &&
    isRecord(value.outputSchema) &&
    isPositiveAtomicAmount(value.amount) &&
    isNonEmptyString(value.asset) &&
    isNonEmptyString(value.network) &&
    value.paymentProtocol === "x402" &&
    value.paymentScheme === "exact" &&
    value.availability === "active" &&
    value.fulfillmentMode === "synchronous_https" &&
    Number.isInteger(value.fulfillmentTimeoutSeconds) &&
    isNonEmptyString(value.updatedAt) &&
    value.authoritativeForPurchase === false &&
    isNonEmptyString(value.canonicalUrl) &&
    isNonEmptyString(value.purchaseSessionEndpoint)
  );
}

function matchesStorefrontRequest(
  manifest: StorefrontManifest,
  sellerSlug: string,
): boolean {
  return (
    manifest.seller.slug === sellerSlug &&
    manifest.products.every(
      (product) =>
        product.sellerId === manifest.sellerId &&
        matchesCanonicalProduct(manifest.canonicalOrigin, sellerSlug, product),
    )
  );
}

function matchesProductRequest(
  document: PublicProductDocument,
  sellerSlug: string,
  productSlug: string,
): boolean {
  return (
    document.sellerSlug === sellerSlug &&
    document.product.productSlug === productSlug &&
    document.product.sellerId === document.sellerId &&
    matchesCanonicalProduct(
      document.canonicalOrigin,
      sellerSlug,
      document.product,
    )
  );
}

function matchesCanonicalProduct(
  canonicalOrigin: string,
  sellerSlug: string,
  product: PublicProduct,
): boolean {
  try {
    const origin = new URL(canonicalOrigin);
    const canonicalUrl = new URL(product.canonicalUrl);
    return (
      canonicalUrl.origin === origin.origin &&
      canonicalUrl.pathname ===
        storefrontProductPath(sellerSlug, product.productSlug)
    );
  } catch {
    return false;
  }
}

function isStorefrontManifest(value: unknown): value is StorefrontManifest {
  return (
    isRecord(value) &&
    value.schemaVersion === discoverySchemaVersion &&
    isNonEmptyString(value.sellerId) &&
    isRecord(value.seller) &&
    isNonEmptyString(value.seller.name) &&
    isNonEmptyString(value.seller.slug) &&
    value.availability === "active" &&
    Number.isInteger(value.publicationRevision) &&
    isNonEmptyString(value.issuedAt) &&
    isNonEmptyString(value.expiresAt) &&
    isNonEmptyString(value.canonicalOrigin) &&
    Array.isArray(value.products) &&
    value.products.every(isPublicProduct)
  );
}

function isStorefrontTombstone(value: unknown): value is StorefrontTombstone {
  return (
    isRecord(value) &&
    value.schemaVersion === discoverySchemaVersion &&
    isNonEmptyString(value.sellerId) &&
    isNonEmptyString(value.sellerSlug) &&
    value.availability === "inactive" &&
    ["suspended", "cancelled", "closed"].includes(String(value.reason)) &&
    Number.isInteger(value.publicationRevision) &&
    isNonEmptyString(value.issuedAt) &&
    isNonEmptyString(value.expiresAt) &&
    isNonEmptyString(value.canonicalOrigin)
  );
}

function isPublicProductDocument(
  value: unknown,
): value is PublicProductDocument {
  return (
    isRecord(value) &&
    value.schemaVersion === productContractSchemaVersion &&
    isNonEmptyString(value.sellerId) &&
    isNonEmptyString(value.sellerSlug) &&
    Number.isInteger(value.publicationRevision) &&
    isNonEmptyString(value.issuedAt) &&
    isNonEmptyString(value.expiresAt) &&
    isNonEmptyString(value.canonicalOrigin) &&
    isPublicProduct(value.product)
  );
}

function isSignedStorefrontManifest(
  value: unknown,
): value is SignedStorefrontManifest {
  return (
    isRecord(value) &&
    isStorefrontManifest(value.document) &&
    isDiscoverySignature(value.signature)
  );
}

function isSignedStorefrontTombstone(
  value: unknown,
): value is SignedStorefrontTombstone {
  return (
    isRecord(value) &&
    isStorefrontTombstone(value.document) &&
    isDiscoverySignature(value.signature)
  );
}

function isSignedPublicProductDocument(
  value: unknown,
): value is SignedPublicProductDocument {
  return (
    isRecord(value) &&
    isPublicProductDocument(value.document) &&
    isDiscoverySignature(value.signature, productContractSchemaVersion)
  );
}
