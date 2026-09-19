import {
  expect,
  installDeterministicWallet,
  registerAndSignInSeller,
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
  await expect(page.locator(".onboarding-complete-row")).toContainText(
    "Storefront created",
    { timeout: 2_000 },
  );

  await page.getByRole("button", { name: "Connect browser wallet" }).click();
  await expect(page.getByText("Payment destination verified")).toBeVisible();
  await expect(page.getByText(walletAddress)).toBeVisible();

  await page
    .getByRole("button", { name: "Create project connection key" })
    .click();
  await expect(
    page.getByText(/Save this project connection key now/),
  ).toBeVisible();
  await expect(page.locator(".credential-secret code")).toContainText("apc2.");

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
