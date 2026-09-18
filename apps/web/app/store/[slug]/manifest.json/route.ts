import { loadStorefrontDiscovery } from "@/features/storefront/controller";

export async function GET(
  _request: Request,
  context: { params: Promise<{ slug: string }> },
) {
  const { slug } = await context.params;
  const state = await loadStorefrontDiscovery(slug);
  if (state.status === "active" || state.status === "inactive") {
    return Response.json(
      {
        document: state.status === "active" ? state.manifest : state.tombstone,
        signature: state.signature,
      },
      {
        status: state.status === "active" ? 200 : 410,
        headers: { "Cache-Control": "public, max-age=30, must-revalidate" },
      },
    );
  }
  return Response.json(
    {
      error: {
        code: state.status === "expired" ? "seller_inactive" : state.reason,
        message:
          state.status === "expired"
            ? "The signed storefront status has expired."
            : "This storefront is unavailable.",
      },
    },
    {
      status: state.status === "expired" ? 410 : 503,
      headers: { "Cache-Control": "no-store" },
    },
  );
}
