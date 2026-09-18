export function browserPurchaseCookieNames(environment = process.env.AGENTPAY_ENV) {
  const secure = environment !== "local";
  return secure
    ? {
        purchase: "__Host-agentpay_purchase",
        csrf: "__Host-agentpay_purchase_csrf",
      }
    : { purchase: "agentpay_purchase", csrf: "agentpay_purchase_csrf" };
}
