import {
  expect,
  expectNoHorizontalOverflow,
  installDeterministicWallet,
  registerAndSignInSeller,
  signInLaunchReadySeller,
  test,
} from "./fixtures/agentpay";

test("seller can register, verify, sign in, and resume onboarding", async ({
  page,
  seed,
}, testInfo) => {
  void seed;
  await registerAndSignInSeller(page, testInfo);
  await expect(
    page.getByRole("heading", { name: "Launch your storefront" }),
  ).toBeVisible();
  await expect(
    page.getByRole("form", { name: "Create storefront" }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Launch your storefront" }),
  ).toBeVisible();
});

test("seller can complete the essential onboarding path and open the dashboard", async ({
  page,
  seed,
}, testInfo) => {
  test.setTimeout(15_000);
  void seed;
  const walletAddress = await installDeterministicWallet(page);
  await registerAndSignInSeller(page, testInfo);

  const slug = `launch-${Date.now().toString(36)}`;
  await page.getByLabel("Storefront name").fill("E2E Launch Storefront");
  await page.getByLabel("Storefront URL name").fill(slug);
  await page.getByLabel("Service API URL").fill("http://127.0.0.1:8090");
  await page.getByRole("button", { name: "Create storefront" }).click();
  await expect(
    page.getByRole("region", { name: "02 Verify your payment destination" }),
  ).toHaveAttribute("aria-current", "step", { timeout: 2_000 });
  await expect(
    page.getByRole("region", { name: "01 Create your storefront" }),
  ).toContainText(`/store/${slug}`);

  await page.getByRole("button", { name: "Connect browser wallet" }).click();
  await expect(
    page.getByRole("region", { name: "03 Create a project connection key" }),
  ).toHaveAttribute("aria-current", "step");
  await expect(
    page.getByRole("region", { name: "02 Verify your payment destination" }),
  ).toContainText(walletAddress);

  await page
    .getByRole("button", { name: "Create project connection key" })
    .click();
  await expect(
    page.getByText(/Save this project connection key now/),
  ).toBeVisible();
  await expect(page.locator(".credential-secret code")).toContainText("apc2.");
  await expect(
    page.getByRole("region", { name: "04 Connect your coding agent" }),
  ).toHaveAttribute("aria-current", "step");
  await expect(
    page.getByRole("region", { name: "05 Run the sandbox purchase" }),
  ).toContainText("Locked");

  await page.goto("/dashboard");
  await expect(
    page.getByRole("heading", { name: "Commerce overview" }),
  ).toBeVisible();
  await expect(
    page.locator("#main-content").getByText("E2E Launch Seller", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Manage products" }),
  ).toBeVisible();
});

test("onboarding layers the launch summary behind the task flow @visual", async ({
  page,
  seed,
}, testInfo) => {
  void seed;
  await registerAndSignInSeller(page, testInfo);

  const summary = page.getByRole("complementary", { name: "Launch progress" });
  const steps = page.locator(".onboarding-steps");
  await expect(summary).toBeVisible();
  await expect(steps).toBeVisible();

  const layerStyles = await page.evaluate(() => {
    const summaryElement = document.querySelector<HTMLElement>(
      ".onboarding-summary",
    );
    const stepsElement =
      document.querySelector<HTMLElement>(".onboarding-steps");
    if (!summaryElement || !stepsElement) {
      return null;
    }
    const summaryStyle = getComputedStyle(summaryElement);
    const stepsStyle = getComputedStyle(stepsElement);
    return {
      summaryPosition: summaryStyle.position,
      summaryZIndex: Number(summaryStyle.zIndex),
      stepsBackground: stepsStyle.backgroundColor,
      stepsPosition: stepsStyle.position,
      stepsZIndex: Number(stepsStyle.zIndex),
      horizontalOverflow:
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth,
    };
  });

  expect(layerStyles).not.toBeNull();
  expect(layerStyles?.summaryPosition).toBe("sticky");
  expect(layerStyles?.stepsPosition).toBe("relative");
  expect(layerStyles?.stepsZIndex).toBeGreaterThan(
    layerStyles?.summaryZIndex ?? 0,
  );
  expect(layerStyles?.stepsBackground).not.toBe("rgba(0, 0, 0, 0)");
  expect(layerStyles?.horizontalOverflow).toBeLessThanOrEqual(1);

  await steps.scrollIntoViewIfNeeded();
  await page.evaluate(() => window.scrollBy(0, 180));
  await expect(
    page.getByRole("region", { name: "01 Create your storefront" }),
  ).toBeVisible();
});

test("seller can sign out from the dashboard", async ({
  page,
  seed,
}, testInfo) => {
  test.setTimeout(15_000);
  void seed;
  await registerAndSignInSeller(page, testInfo);
  const signOut = page.getByRole("button", { name: "Sign out" });
  await expect(signOut).toBeVisible({ timeout: 1_500 });
  await signOut.click();
  await expect(page).toHaveURL(/\/sign-in$/);
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/sign-in/);
});

test("seller test purchase passes three clean reset rehearsals", async ({
  page,
  request,
}) => {
  test.setTimeout(45_000);
  for (let rehearsal = 1; rehearsal <= 3; rehearsal += 1) {
    const reset = await request.post(
      "http://127.0.0.1:8080/__dev/seed-profile/reset",
    );
    expect(reset.ok(), `rehearsal ${rehearsal} seed reset`).toBeTruthy();
    await page.context().clearCookies();
    await signInLaunchReadySeller(page);
    await page.goto("/dashboard/onboarding#test-purchase");
    const testPurchase = page.getByRole("region", {
      name: "Run test purchase",
    });
    await expect(testPurchase).toBeVisible();
    await testPurchase
      .getByRole("button", { name: "Run test purchase" })
      .click();
    await expect(testPurchase.getByText("Launch test passed")).toBeVisible();
    await expect(testPurchase.getByText("Local mock payment")).toBeVisible();
    await expect(
      testPurchase.getByRole("link", { name: "Open verified transaction" }),
    ).toBeVisible();
    await expect(
      testPurchase.getByRole("listitem", { name: "Dashboard reconciled" }),
    ).toContainText("Passed");
  }
});

test("seller test purchase remains usable at supported widths @visual", async ({
  page,
}) => {
  await signInLaunchReadySeller(page);
  await page.goto("/dashboard/onboarding#test-purchase");
  const testPurchase = page.getByRole("region", { name: "Run test purchase" });
  await expect(testPurchase).toBeVisible();
  await expect(
    testPurchase.getByRole("button", { name: "Run test purchase" }),
  ).toBeVisible();
  await expectNoHorizontalOverflow(page);
});
