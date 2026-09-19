import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readText = (path) => readFile(path, "utf8");
const readJson = async (path) => JSON.parse(await readText(path));

test("repository declares the proprietary ownership boundary", async () => {
  const license = await readText("LICENSE");

  assert.match(license, /Copyright \(c\) 2026 Pratham Ranka and Ayush Garg/);
  assert.match(license, /All rights reserved/);
  assert.match(license, /GitHub Terms of Service/);
  assert.match(license, /third-party components/i);
});

test("publishable metadata cannot imply an open-source grant", async () => {
  const npmPackages = await Promise.all([
    readJson("package.json"),
    readJson("apps/web/package.json"),
    readJson("packages/local-mcp-connector/package.json"),
    readJson("verification/node/package.json"),
  ]);

  for (const npmPackage of npmPackages) {
    assert.equal(npmPackage.private, true, `${npmPackage.name} must remain private`);
    assert.equal(npmPackage.license, "UNLICENSED", `${npmPackage.name} must be unlicensed`);
  }

  const pythonVerifier = await readText("verification/python/pyproject.toml");
  const recommendationWorkspace = await readText("rl/pyproject.toml");
  const dotnetVerifier = await readText(
    "verification/dotnet/AgentPay.Verify/AgentPay.Verify.csproj",
  );

  assert.match(pythonVerifier, /license = "LicenseRef-Proprietary"/);
  assert.match(recommendationWorkspace, /license = "LicenseRef-Proprietary"/);
  assert.match(dotnetVerifier, /<IsPackable>false<\/IsPackable>/);
});

test("Vercel Analytics remains pinned and disclosed", async () => {
  const webPackage = await readJson("apps/web/package.json");
  const privacyNotice = await readText("apps/web/app/privacy/page.tsx");

  assert.equal(webPackage.dependencies["@vercel/analytics"], "2.0.1");
  assert.match(privacyNotice, /Vercel Web Analytics is enabled across the site/);
  assert.match(privacyNotice, /does not use third-party cookies/);
  assert.match(privacyNotice, /No custom analytics events are currently configured/);
});

test("repository ownership and contribution controls remain discoverable", async () => {
  const codeowners = await readText(".github/CODEOWNERS");
  const contributing = await readText("CONTRIBUTING.md");
  const security = await readText("SECURITY.md");
  const readme = await readText("README.md");
  const governance = await readText("docs/REPOSITORY_GOVERNANCE.md");
  const productTerms = await readText("apps/web/app/terms/page.tsx");

  assert.match(codeowners, /^\* @PrathamRanka @gargayush1911$/m);
  assert.match(contributing, /Signed-off-by:/);
  assert.match(contributing, /retain ownership/i);
  assert.match(security, /private vulnerability reporting/i);
  assert.match(security, /Do not open a public issue/i);
  assert.match(readme, /## Ownership and license/);
  assert.match(governance, /cannot make a public repository technically uncloneable/i);
  assert.match(governance, /Make the repository private/i);
  assert.match(productTerms, /AgentPay software, site, documentation, and branding are proprietary/);
});

test("GitHub contribution templates route changes through review", async () => {
  const pullRequestTemplate = await readText(".github/pull_request_template.md");
  const issueConfiguration = await readText(".github/ISSUE_TEMPLATE/config.yml");
  const bugTemplate = await readText(".github/ISSUE_TEMPLATE/bug.yml");
  const featureTemplate = await readText(".github/ISSUE_TEMPLATE/feature.yml");

  assert.match(pullRequestTemplate, /I have the right to submit this contribution/);
  assert.match(pullRequestTemplate, /Signed-off-by/);
  assert.match(issueConfiguration, /blank_issues_enabled: false/);
  assert.match(bugTemplate, /name: Bug report/);
  assert.match(featureTemplate, /name: Feature request/);
});

test("continuous integration enforces the governance contract", async () => {
  const workflow = await readText(".github/workflows/ci.yml");

  assert.match(workflow, /node --test scripts\/repository-governance\.test\.mjs/);
});
