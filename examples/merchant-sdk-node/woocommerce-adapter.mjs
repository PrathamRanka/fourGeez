import { createWooCommerceAdapter } from "../../packages/merchant-sdk/dist/index.js";

export const wooCommerceAdapter = createWooCommerceAdapter({
  storeOrigin: requiredEnvironment("WOOCOMMERCE_STORE_ORIGIN"),
  consumerKey: requiredEnvironment("WOOCOMMERCE_CONSUMER_KEY"),
  consumerSecret: requiredEnvironment("WOOCOMMERCE_CONSUMER_SECRET"),
});

function requiredEnvironment(name) {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
