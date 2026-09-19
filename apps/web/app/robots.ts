import type { MetadataRoute } from "next";
import { buildMarketingRobots } from "@/features/marketing/seo";

export default function robots(): MetadataRoute.Robots {
  return buildMarketingRobots();
}
