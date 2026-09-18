import { loadStorefrontManifest } from "@/features/storefront/controller";

export async function GET(_request: Request, context: { params: Promise<{ slug: string }> }) {
  const { slug } = await context.params;
  const manifest = await loadStorefrontManifest(slug);
  return manifest
    ? Response.json(manifest, { headers: { "Cache-Control": "public, max-age=60" } })
    : Response.json({ error: "Storefront not found." }, { status: 404 });
}
