import {
  expect,
  expectNoHorizontalOverflow,
  expectNoSeriousAccessibilityViolations,
  test,
} from "./fixtures/agentpay";

test("public navigation and legal pages resolve", async ({ page, seed }) => {
  void seed;
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Sell your API to AI agents." }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Sign in" }).first(),
  ).toHaveAttribute("href", "/sign-in");

  for (const path of ["/docs", "/privacy", "/terms", "/security"]) {
    const response = await page.goto(path);
    expect(response?.ok(), `${path} must resolve`).toBeTruthy();
    await expect(page.locator("main")).toBeVisible();
  }
});

test("historical buyer approval runtime is not exposed in Lean V1", async ({
  page,
  seed,
}) => {
  void seed;
  const response = await page.goto(
    "/approve/aps_01ARZ3NDEKTSV4RRFFQ69G5FAV?token=historical-token",
  );
  expect(response?.status()).toBe(404);
  await expect(page.getByText("This page could not be found.")).toBeVisible();

  const snapshot = await page.request.get(
    "/approve/aps_01ARZ3NDEKTSV4RRFFQ69G5FAV/snapshot?token=historical-token",
  );
  expect(snapshot.status()).toBe(404);
  const decision = await page.request.post(
    "/approve/aps_01ARZ3NDEKTSV4RRFFQ69G5FAV/decision?token=historical-token",
    { data: { decision: "approve" } },
  );
  expect(decision.status()).toBe(404);
});

test("storefront discovery and agent inspection use the signed launch catalog", async ({
  page,
  seed,
}) => {
  test.setTimeout(20_000);
  void seed;
  await page.goto("/store/demo-seller");
  await expect(
    page.getByRole("heading", { name: "Northstar Research" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Market Snapshot" }),
  ).toBeVisible();
  await page
    .getByRole("article")
    .filter({
      has: page.getByRole("heading", { name: "Market Snapshot" }),
    })
    .getByRole("link", { name: "View product" })
    .click();
  await expect(page).toHaveURL(
    /\/store\/demo-seller\/products\/market-snapshot$/,
  );
  await expect(
    page.getByRole("heading", { name: "Market Snapshot" }),
  ).toBeVisible();

  const manifest = await page.request.get("/store/demo-seller/manifest.json");
  expect(manifest.ok()).toBeTruthy();
  const manifestBody = (await manifest.json()) as {
    signature?: { value?: string };
  };
  expect(manifestBody.signature?.value).toBeTruthy();

  await page.goto("/demo/agent-checkout");
  await page.getByLabel("Storefront slug").fill("demo-seller");
  await page
    .getByLabel("What do you need?")
    .fill("A concise market snapshot within the listed budget");
  await page.getByRole("button", { name: "Inspect storefront" }).click();
  await expect(page.getByText("getStorefrontManifest")).toBeVisible();
  await expect(
    page.getByText(/deterministic fallback selected Market Snapshot/i),
  ).toBeVisible();
});

test("keyboard basics and critical-page accessibility pass", async ({
  page,
  seed,
}) => {
  void seed;
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toHaveAttribute("href", "#main-content");
  await page.keyboard.press("Enter");
  await expect(page.locator("#main-content")).toBeVisible();
  expect(
    await page.evaluate(
      () => matchMedia("(prefers-reduced-motion: reduce)").matches,
    ),
  ).toBeTruthy();
  await expect(page.locator(".client-ribbon-track").first()).toHaveCSS(
    "animation-name",
    "none",
  );
  await expectNoSeriousAccessibilityViolations(page);

  await page.goto("/sign-in");
  await expectNoSeriousAccessibilityViolations(page);
  await page.goto("/store/demo-seller");
  await expectNoSeriousAccessibilityViolations(page);
});

test("@visual public surfaces render without overflow and produce responsive screenshots", async ({
  page,
  seed,
}, testInfo) => {
  void seed;
  const width = page.viewportSize()?.width ?? 0;
  for (const [name, path] of [
    ["landing", "/"],
    ["storefront", "/store/demo-seller"],
    ["product", "/store/demo-seller/products/market-snapshot"],
    ["sign-in", "/sign-in"],
  ] as const) {
    await page.goto(path);
    await expect(page.locator("main")).toBeVisible();
    await expectNoHorizontalOverflow(page);
    const screenshotPath = testInfo.outputPath(`${name}-${width}.png`);
    await page.screenshot({ path: screenshotPath, fullPage: true });
    await testInfo.attach(`${name}-${width}`, {
      path: screenshotPath,
      contentType: "image/png",
    });
  }
});
