import { createShopifyAdminAdapter } from "../../packages/merchant-sdk/dist/index.js";

export const shopifyAdapter = createShopifyAdminAdapter({
  shopDomain: requiredEnvironment("SHOPIFY_SHOP_DOMAIN"),
  apiVersion: requiredEnvironment("SHOPIFY_ADMIN_API_VERSION"),
  accessToken: requiredEnvironment("SHOPIFY_ADMIN_ACCESS_TOKEN"),
});

function requiredEnvironment(name) {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
