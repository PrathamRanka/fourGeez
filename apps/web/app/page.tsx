import type { Metadata } from "next";
import {
  marketingMetadata,
  marketingStructuredData,
} from "@/features/marketing/seo";
import { MarketingPage } from "@/features/marketing/view/marketing-page";

export const metadata: Metadata = marketingMetadata;

// HomePage coordinates the server-rendered public marketing experience.
export default function HomePage() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify(marketingStructuredData).replace(
            /</g,
            "\\u003c",
          ),
        }}
      />
      <MarketingPage />
    </>
  );
}
