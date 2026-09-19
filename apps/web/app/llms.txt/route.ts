import { marketingLlmsText } from "@/features/marketing/seo";

export function GET(): Response {
  return new Response(marketingLlmsText, {
    headers: {
      "Cache-Control": "public, max-age=3600, s-maxage=86400",
      "Content-Type": "text/plain; charset=utf-8",
    },
  });
}
