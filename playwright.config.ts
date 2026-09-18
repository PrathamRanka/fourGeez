import { defineConfig } from "@playwright/test";

const webPort = process.env.AGENTPAY_LOCAL_WEB_PORT ?? "3000";
const baseURL =
  process.env.AGENTPAY_E2E_BASE_URL ?? `http://localhost:${webPort}`;
const apiOrigin =
  process.env.AGENTPAY_E2E_API_ORIGIN ?? "http://127.0.0.1:8080";
const localBrowserChannel =
  process.env.PLAYWRIGHT_CHANNEL ??
  (process.platform === "win32" && !process.env.CI ? "chrome" : undefined);

const sharedUse = {
  baseURL,
  channel: localBrowserChannel,
  colorScheme: "light" as const,
  reducedMotion: "reduce" as const,
  screenshot: "only-on-failure" as const,
  trace: "retain-on-failure" as const,
  video: "off" as const,
};

export default defineConfig({
  testDir: "./e2e",
  outputDir: "./.cache/e2e/artifacts",
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 45_000,
  expect: { timeout: 10_000 },
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: ".cache/e2e/report" }],
  ],
  webServer: process.env.PLAYWRIGHT_SKIP_WEBSERVER
    ? undefined
    : {
        // Launch the supervisor directly. On Windows, wrapping it in npm adds
        // cmd/npm process layers that can keep Playwright's webServer teardown
        // waiting after every test has passed.
        command: "node scripts/dev-local.mjs",
        url: `${apiOrigin}/health/ready`,
        timeout: 120_000,
        reuseExistingServer: !process.env.CI,
        stdout: "pipe",
        stderr: "pipe",
      },
  projects: [
    {
      name: "chromium-functional",
      grepInvert: /@visual/,
      use: { ...sharedUse, viewport: { width: 1280, height: 900 } },
    },
    {
      name: "responsive-360",
      grep: /@visual/,
      use: { ...sharedUse, viewport: { width: 360, height: 800 } },
    },
    {
      name: "responsive-768",
      grep: /@visual/,
      use: { ...sharedUse, viewport: { width: 768, height: 1024 } },
    },
    {
      name: "responsive-1280",
      grep: /@visual/,
      use: { ...sharedUse, viewport: { width: 1280, height: 900 } },
    },
    {
      name: "responsive-1440",
      grep: /@visual/,
      use: { ...sharedUse, viewport: { width: 1440, height: 1000 } },
    },
  ],
});
