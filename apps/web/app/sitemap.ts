import type { MetadataRoute } from "next";
import { buildMarketingSitemap } from "@/features/marketing/seo";

export default function sitemap(): MetadataRoute.Sitemap {
  return buildMarketingSitemap();
}
