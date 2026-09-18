import AxeBuilder from "@axe-core/playwright";
import {
  expect,
  test as base,
  type Page,
  type TestInfo,
} from "@playwright/test";
import { Wallet } from "ethers";

const apiOrigin =
  process.env.AGENTPAY_E2E_API_ORIGIN ?? "http://127.0.0.1:8080";

export type LaunchReadySeed = {
  profileName: "launch-ready";
  launchReadySellerId: string;
  incompleteSellerId: string;
  belowThresholdRouteId: string;
  approvalRequiredRouteId: string;
  paymentPendingTransactionId: string;
  fulfilledTransactionId: string;
  failedTransactionId: string;
  disputedTransactionId: string;
  validEvidenceTransactionId: string;
  invalidEvidenceTransactionId: string;
};

type AgentPayFixtures = { seed: LaunchReadySeed };

export const test = base.extend<AgentPayFixtures>({
  seed: async ({ request }, use) => {
    const resetResponse = await request.post(
      `${apiOrigin}/__dev/seed-profile/reset`,
    );
    expect(
      resetResponse.ok(),
      "launch-ready seed reset must succeed",
    ).toBeTruthy();
    const metadataResponse = await request.get(
      `${apiOrigin}/__dev/seed-profile`,
    );
    expect(
      metadataResponse.ok(),
      "launch-ready seed metadata must be readable",
    ).toBeTruthy();
    const metadata = (await metadataResponse.json()) as LaunchReadySeed;
    expect(metadata.profileName).toBe("launch-ready");
    await use(metadata);
  },
});

export { expect };

export async function registerAndSignInSeller(page: Page, testInfo: TestInfo) {
  const uniqueSuffix = `${testInfo.workerIndex}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const account = {
    email: `launch-${uniqueSuffix}@example.test`,
    name: "E2E Launch Seller",
    password: "correct-horse-battery-staple",
  };

  await page.goto("/sign-up");
  await page.getByLabel("Your name").fill(account.name);
  await page.getByLabel("Work email").fill(account.email);
  await page.getByLabel("Password").fill(account.password);
  const createAccount = page.getByRole("button", {
    name: "Create seller account",
  });
  await expect(createAccount).toBeEnabled();
  await createAccount.click();

  const developmentCode = await page
    .locator(".auth-development-code code")
    .textContent();
  expect(developmentCode).toMatch(/^\d{6}$/);
  await page.getByRole("button", { name: "Continue to verification" }).click();
  await expect(page).toHaveURL(/\/verify/);
  await page.getByLabel("Verification code").fill(developmentCode ?? "");
  await page.getByRole("button", { name: "Verify email" }).click();
  await expect(page).toHaveURL(/\/sign-in/);

  await page.getByLabel("Work email").fill(account.email);
  await page.getByLabel("Password").fill(account.password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/dashboard\/onboarding/);
  return account;
}

export async function installDeterministicWallet(page: Page) {
  const wallet = Wallet.createRandom();
  await page.exposeFunction("agentPayWalletAddress", () => wallet.address);
  await page.exposeFunction("agentPaySignMessage", (message: string) =>
    wallet.signMessage(message),
  );
  await page.addInitScript(() => {
    const bridge = window as typeof window & {
      agentPayWalletAddress: () => Promise<string>;
      agentPaySignMessage: (message: string) => Promise<string>;
      ethereum?: {
        request: (request: {
          method: string;
          params?: unknown[];
        }) => Promise<unknown>;
      };
    };
    bridge.ethereum = {
      async request(request) {
        if (request.method === "eth_requestAccounts") {
          return [await bridge.agentPayWalletAddress()];
        }
        if (request.method === "personal_sign") {
          return bridge.agentPaySignMessage(String(request.params?.[0] ?? ""));
        }
        throw new Error(`Unsupported E2E wallet method: ${request.method}`);
      },
    };
  });
  return wallet.address;
}

export async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth -
      document.documentElement.clientWidth,
  );
  expect(overflow, "page must not overflow horizontally").toBeLessThanOrEqual(
    1,
  );
}

export async function expectNoSeriousAccessibilityViolations(page: Page) {
  const result = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .analyze();
  const blocking = result.violations.filter(
    (violation) =>
      violation.impact === "critical" || violation.impact === "serious",
  );
  expect(
    blocking,
    blocking
      .map(
        (violation) =>
          `${violation.id}: ${violation.help} (${violation.nodes.length})`,
      )
      .join("\n"),
  ).toEqual([]);
}
