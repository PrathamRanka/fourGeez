import { expect, test } from "./fixtures/agentpay";

test("browser storefront completes x402 payment, fulfillment, receipt, evidence, and dispute", async ({
  page,
  seed,
}) => {
  test.setTimeout(20_000);
  void seed;
  await page.goto("/store/demo-seller/products/market-snapshot");
  await page.getByRole("checkbox", { name: /confirm/i }).check();
  await page.getByRole("button", { name: "Review exact payment" }).click();
  await expect(page.getByText("Payment ready")).toBeVisible();
  await page
    .getByRole("button", { name: "Complete local demo payment" })
    .click();
  await expect(page.getByText("Paid and fulfilled")).toBeVisible();
  await expect(page.getByRole("link", { name: /View receipt/i })).toBeVisible();
  await expect(
    page.getByRole("link", { name: /View evidence/i }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /Open dispute/i }),
  ).toBeVisible();
});

test("agent demo completes below-threshold x402 purchase and fulfillment", async ({
  page,
  seed,
}) => {
  test.setTimeout(20_000);
  void seed;
  await page.goto("/demo/agent-checkout");
  await expect(page.getByLabel("Storefront slug")).toBeVisible({
    timeout: 1_500,
  });
  await page.getByLabel("Storefront slug").fill("demo-seller");
  await page
    .getByLabel("What do you need?")
    .fill("Buy the Market Snapshot within 3 USDC");
  await page.getByRole("button", { name: "Inspect storefront" }).click();
  await expect(
    page.getByRole("heading", { name: "Agent-selected checkout" }),
  ).toBeVisible();
  await page.getByRole("checkbox", { name: /confirm/i }).check();
  await page.getByRole("button", { name: "Review exact payment" }).click();
  await expect(page.getByText("Payment ready")).toBeVisible();
  await page
    .getByRole("button", { name: "Complete local demo payment" })
    .click();
  await expect(page.getByText("Paid and fulfilled")).toBeVisible();
  await expect(page.getByText(/concise market snapshot/i)).toBeVisible();
});

test("cancellation and credential revocation deny stale discovery and new purchases", async ({
  request,
  page,
  seed,
}) => {
  test.setTimeout(10_000);
  const cancellation = await request.post(
    `http://127.0.0.1:8080/__dev/seed-profile/sellers/${seed.launchReadySellerId}/cancel`,
  );
  expect(cancellation.ok()).toBeTruthy();
  expect(await cancellation.json()).toMatchObject({
    entitlementStatus: "cancelled",
    credentialRevoked: true,
  });
  const manifest = await request.get(
    "http://127.0.0.1:8080/store/demo-seller/manifest.json",
  );
  expect(manifest.status()).toBe(410);
  await page.goto("/store/demo-seller");
  await expect(page.getByText(/not accepting new purchases/i)).toBeVisible();
});
