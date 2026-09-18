import { loadStorefrontLLMSText } from "@/features/storefront/controller";

export async function GET(_request: Request, context: { params: Promise<{ slug: string }> }) {
  const { slug } = await context.params;
  const document = await loadStorefrontLLMSText(slug);
  return document
    ? new Response(document, { headers: { "Content-Type": "text/plain; charset=utf-8" } })
    : new Response("Storefront not found.\n", { status: 404 });
}
