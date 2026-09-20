import { loadStorefrontLLMSText } from "@/features/storefront/controller";

export async function GET(
  _request: Request,
  context: { params: Promise<{ slug: string }> },
) {
  const { slug } = await context.params;
  const result = await loadStorefrontLLMSText(slug);
  if (!result.ok) {
    return new Response("Storefront discovery is unavailable.\n", {
      status: result.status === 410 ? 410 : result.status === 404 ? 404 : 503,
      headers: { "Cache-Control": "no-store" },
    });
  }
  return new Response(result.value, {
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=0, must-revalidate",
    },
  });
}
