import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const workflowUrl = new URL(
  "../.github/workflows/seller-package-release.yml",
  import.meta.url,
);
const ciWorkflowUrl = new URL("../.github/workflows/ci.yml", import.meta.url);
const immutableActionReference =
  /^\s*- uses:\s+[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+@[a-f0-9]{40}(?:\s+#\s+v\d+(?:\.\d+(?:\.\d+)?)?)?$/u;

test("seller package releases are manual, reviewed, and least privilege", async () => {
  const workflow = await readFile(workflowUrl, "utf8");

  assert.match(workflow, /workflow_dispatch:/u);
  assert.doesNotMatch(workflow, /(?:^|\n)\s+push:/u);
  assert.match(workflow, /environment:\s*seller-package-release/u);
  assert.match(workflow, /build:[\s\S]*?contents:\s*read/u);
  assert.match(workflow, /release:[\s\S]*?contents:\s*write/u);
  assert.match(workflow, /needs:\s*build/u);
  assert.match(workflow, /source_commit:/u);
  assert.match(workflow, /git rev-parse HEAD/u);
  assert.match(workflow, /npm run test:package-distribution/u);
  assert.match(workflow, /--expected-tag/u);
  assert.match(workflow, /--expected-commit/u);
  assert.doesNotMatch(workflow, /--expected-commit\s+"?\$GITHUB_SHA/u);
  assert.match(workflow, /sha256sum --check SHA256SUMS/u);
  assert.match(workflow, /jq -er/u);
  assert.match(workflow, /gh release create[\s\S]*--draft[\s\S]*--verify-tag/u);
  assert.match(workflow, /--latest=false/u);
  assert.match(workflow, /gh release edit[\s\S]*--draft=false/u);
  assert.doesNotMatch(workflow, /npm publish/u);
  assertAllActionReferencesAreImmutable(workflow);
});

test("new package release CI action references are immutable", async () => {
  const workflow = (await readFile(ciWorkflowUrl, "utf8")).replaceAll(
    "\r\n",
    "\n",
  );
  const packageReleaseJob = workflow.match(
    /(?:^|\n)  package-release:\n([\s\S]*?)(?=\n  web:\n)/u,
  )?.[1];

  assert.ok(packageReleaseJob, "package-release job must exist");
  assertAllActionReferencesAreImmutable(packageReleaseJob);
});

function assertAllActionReferencesAreImmutable(workflow) {
  const actionReferences =
    workflow.match(/^[ \t]*- uses:[ \t]+\S+.*$/gmu) ?? [];
  assert.notEqual(actionReferences.length, 0, "workflow must use actions");
  for (const actionReference of actionReferences) {
    assert.match(actionReference, immutableActionReference);
  }
}
