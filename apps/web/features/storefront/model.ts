import type { PaidRoute } from "@/features/products/model";

export type StorefrontManifest = {
  seller: { name: string; slug: string };
  routes: PaidRoute[];
};

export function storefrontProductPath(slug: string, productSlug: string): string {
  return `/store/${encodeURIComponent(slug)}/products/${encodeURIComponent(productSlug)}`;
}

export function paidRouteUrl(
  apiOrigin: string,
  slug: string,
  pathPattern: string,
): string {
  return `${apiOrigin.replace(/\/$/, "")}/pay/${encodeURIComponent(slug)}${pathPattern}`;
}
