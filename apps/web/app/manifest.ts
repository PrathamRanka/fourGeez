import type { MetadataRoute } from "next";
import { buildMarketingManifest } from "@/features/marketing/seo";

export default function manifest(): MetadataRoute.Manifest {
  return {
    ...buildMarketingManifest(),
    icons: [
      {
        src: "/brand/agentpay-icon-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/brand/agentpay-icon-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
