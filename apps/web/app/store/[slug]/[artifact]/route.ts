import { loadStorefrontDiscovery } from "@/features/storefront/controller";
import { storefrontProductPath } from "@/features/storefront/model";

// GET serves dynamic storefront artifacts that cannot use Next's static metadata convention.
export async function GET(
  _request: Request,
  context: { params: Promise<{ slug: string; artifact: string }> },
) {
  const { slug, artifact } = await context.params;
  if (artifact !== "sitemap.xml") {
    return new Response("Not found.\n", { status: 404 });
  }
  const state = await loadStorefrontDiscovery(slug);
  if (state.status !== "active") {
    return new Response("Storefront not found.\n", { status: 404 });
  }
  const origin = (
    process.env.AGENTPAY_WEB_ORIGIN ?? "http://localhost:3000"
  ).replace(/\/$/, "");
  const urls = [
    `${origin}/store/${slug}`,
    ...state.manifest.products.map(
      (product) =>
        `${origin}${storefrontProductPath(slug, product.productSlug)}`,
    ),
  ];
  const xml = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">${urls.map((url) => `<url><loc>${url}</loc></url>`).join("")}</urlset>\n`;
  return new Response(xml, {
    headers: { "Content-Type": "application/xml; charset=utf-8" },
  });
}
