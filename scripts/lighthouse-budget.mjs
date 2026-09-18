import { mkdir, readFile, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { launch } from "chrome-launcher";
import lighthouse from "lighthouse";

const root = resolve(import.meta.dirname, "..");
const budgetPath = resolve(root, "quality", "lighthouse-budgets.json");
const budget = JSON.parse(await readFile(budgetPath, "utf8"));
const baseURL = process.env.AGENTPAY_E2E_BASE_URL ?? "http://localhost:3000";
const chromeProfile = resolve(root, ".cache", `lighthouse-${process.pid}`);

async function assertRuntimeReady() {
  const healthOrigin =
    process.env.AGENTPAY_E2E_API_ORIGIN ?? "http://127.0.0.1:8080";
  const response = await fetch(new URL("/health/ready", healthOrigin), {
    signal: AbortSignal.timeout(5_000),
  });
  if (!response.ok) {
    throw new Error(
      "AgentPay local runtime is not ready. Start it with npm run dev:local.",
    );
  }
}

await assertRuntimeReady();
await mkdir(chromeProfile, { recursive: true });
const chrome = await launch({
  chromePath: process.env.CHROME_PATH,
  chromeFlags: ["--headless=new", "--disable-gpu", "--no-sandbox"],
  userDataDir: chromeProfile,
});

let failed = false;
try {
  for (const route of budget.routes) {
    const auditedURL = new URL(route.path, baseURL).toString();
    const result = await lighthouse(auditedURL, {
      port: chrome.port,
      output: "json",
      logLevel: "error",
      onlyCategories: ["accessibility", "best-practices", "performance"],
    });
    if (!result?.lhr) {
      throw new Error(`Lighthouse returned no result for ${route.name}.`);
    }

    const scores = Object.fromEntries(
      Object.entries(result.lhr.categories).map(([name, category]) => [
        name,
        category.score ?? 0,
      ]),
    );
    process.stdout.write(`${route.name}: ${JSON.stringify(scores)}\n`);
    for (const [category, minimum] of Object.entries(budget.minimumScores)) {
      if ((scores[category] ?? 0) < minimum) {
        failed = true;
        process.stderr.write(
          `${route.name} ${category} score ${scores[category] ?? 0} is below ${minimum}.\n`,
        );
      }
    }
  }
} finally {
  try {
    chrome.kill();
  } catch (error) {
    process.stderr.write(
      `Lighthouse completed, but Chrome cleanup reported: ${error instanceof Error ? error.message : String(error)}\n`,
    );
  }
  try {
    await rm(chromeProfile, {
      force: true,
      maxRetries: 0,
      recursive: true,
    });
  } catch (error) {
    process.stderr.write(
      `Lighthouse profile cleanup was deferred: ${error instanceof Error ? error.message : String(error)}\n`,
    );
  }
}

// Lighthouse leaves protocol timers alive on some Windows Chrome builds even
// after the audited browser is killed. This is a bounded CLI, so exit only
// after every audit and cleanup attempt has completed.
process.exit(failed ? 1 : 0);
