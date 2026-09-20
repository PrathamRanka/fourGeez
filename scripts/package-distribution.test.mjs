import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import {
  mkdtemp,
  mkdir,
  readFile,
  readdir,
  rm,
  stat,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { promisify } from "node:util";
import test from "node:test";

const execFileAsync = promisify(execFile);
const repositoryRoot = path.resolve(import.meta.dirname, "..");
const npmCli =
  process.env.npm_execpath ??
  path.join(
    path.dirname(process.execPath),
    "node_modules",
    "npm",
    "bin",
    "npm-cli.js",
  );
const npmEnvironment = {
  ...process.env,
  npm_config_cache: path.join(
    repositoryRoot,
    ".cache",
    "npm-package-release-tests",
  ),
  npm_config_update_notifier: "false",
};
const expectedArtifacts = [
  "agentpay-local-mcp-connector-0.1.0.tgz",
  "agentpay-merchant-sdk-0.1.0.tgz",
];

test("seller packages produce reproducible self-contained release artifacts", async (context) => {
  const temporaryRoot = await mkdtemp(
    path.join(tmpdir(), "agentpay-package-release-"),
  );
  context.after(() => rm(temporaryRoot, { recursive: true, force: true }));
  const sourceDirty = await isWorktreeDirty();

  const firstOutput = path.join(temporaryRoot, "first");
  const secondOutput = path.join(temporaryRoot, "second");
  await buildRelease(firstOutput);
  await buildRelease(secondOutput);

  for (const artifactName of expectedArtifacts) {
    const firstArtifact = path.join(firstOutput, artifactName);
    const secondArtifact = path.join(secondOutput, artifactName);
    assert.equal(await sha256(firstArtifact), await sha256(secondArtifact));
  }

  await verifyReleaseMetadata(firstOutput, sourceDirty);
  await verifyReleaseCommand(firstOutput);
  await verifyCleanInstall(firstOutput, temporaryRoot);
  await verifyTamperDetection(secondOutput);
  await verifyDirtyWorktreeRefusal(temporaryRoot);
});

async function buildRelease(outputDirectory) {
  await execFileAsync(
    process.execPath,
    [
      "scripts/build-package-release.mjs",
      "--output",
      outputDirectory,
      "--allow-dirty",
    ],
    { cwd: repositoryRoot, timeout: 120_000 },
  );
}

async function verifyReleaseMetadata(outputDirectory, expectedDirty) {
  const checksumLines = (
    await readFile(path.join(outputDirectory, "SHA256SUMS"), "utf8")
  )
    .trim()
    .split(/\r?\n/u);
  assert.equal(checksumLines.length, expectedArtifacts.length);

  const provenance = JSON.parse(
    await readFile(path.join(outputDirectory, "provenance.json"), "utf8"),
  );
  assert.equal(provenance.schemaVersion, "agentpay.package-release.v1");
  assert.match(provenance.source.commit, /^[a-f0-9]{40}$/u);
  assert.equal(provenance.source.dirty, expectedDirty);
  assert.deepEqual(
    provenance.artifacts.map((artifact) => artifact.fileName),
    expectedArtifacts,
  );

  for (const artifact of provenance.artifacts) {
    assert.equal(
      artifact.sha256,
      await sha256(path.join(outputDirectory, artifact.fileName)),
    );
    assert.equal(
      checksumLines.includes(`${artifact.sha256}  ${artifact.fileName}`),
      true,
    );
    assert.equal(artifact.packedFiles.some(isForbiddenPackedPath), false);
  }
  const connector = provenance.artifacts.find(
    (artifact) => artifact.packageName === "@agentpay/local-mcp-connector",
  );
  assert.equal(connector.packedFiles.includes("dist/cli.js"), true);
  const merchant = provenance.artifacts.find(
    (artifact) => artifact.packageName === "@agentpay/merchant-sdk",
  );
  assert.equal(merchant.packedFiles.includes("dist/index.js"), true);
  assert.equal(merchant.packedFiles.includes("dist/dynamodb.js"), true);
  assert.equal(
    merchant.packedFiles.includes(
      "node_modules/@agentpay/verify-node/dist/index.js",
    ),
    true,
  );
}

async function verifyDirtyWorktreeRefusal(temporaryRoot) {
  const outputDirectory = path.join(temporaryRoot, "dirty-release");
  const dirtyMarker = path.join(
    repositoryRoot,
    `package-release-dirty-test-${process.pid}.txt`,
  );
  await writeFile(dirtyMarker, "intentional dirty-worktree fixture\n");
  try {
    const { stdout: status } = await execFileAsync(
      "git",
      ["status", "--porcelain", "--untracked-files=all", "--", dirtyMarker],
      { cwd: repositoryRoot, timeout: 10_000 },
    );
    assert.notEqual(
      status.trim(),
      "",
      "dirty-worktree fixture must be visible to Git",
    );
    await assert.rejects(
      execFileAsync(
        process.execPath,
        ["scripts/build-package-release.mjs", "--output", outputDirectory],
        { cwd: repositoryRoot, timeout: 30_000 },
      ),
      (error) => {
        assert.match(error.stderr, /clean worktree/u);
        return true;
      },
    );
  } finally {
    await rm(dirtyMarker, { force: true });
  }
}

async function isWorktreeDirty() {
  const { stdout } = await execFileAsync(
    "git",
    ["status", "--porcelain", "--untracked-files=all"],
    { cwd: repositoryRoot, timeout: 10_000 },
  );
  return stdout.trim() !== "";
}

async function verifyReleaseCommand(outputDirectory) {
  await execFileAsync(
    process.execPath,
    ["scripts/verify-package-release.mjs", "--directory", outputDirectory],
    { cwd: repositoryRoot, timeout: 30_000 },
  );
}

async function verifyTamperDetection(outputDirectory) {
  const artifactPath = path.join(outputDirectory, expectedArtifacts[0]);
  const original = await readFile(artifactPath);
  await writeFile(
    artifactPath,
    Buffer.concat([original, Buffer.from("tampered")]),
  );
  await assert.rejects(
    execFileAsync(
      process.execPath,
      ["scripts/verify-package-release.mjs", "--directory", outputDirectory],
      { cwd: repositoryRoot, timeout: 30_000 },
    ),
    /Checksum verification failed/u,
  );
}

async function verifyCleanInstall(outputDirectory, temporaryRoot) {
  for (const artifactName of expectedArtifacts) {
    const consumerDirectory = path.join(
      temporaryRoot,
      artifactName.replace(/\.tgz$/u, ""),
    );
    await mkdir(consumerDirectory, { recursive: true });
    await writeFile(
      path.join(consumerDirectory, "package.json"),
      JSON.stringify({ name: "agentpay-package-consumer", private: true }),
    );
    await execFileAsync(
      process.execPath,
      [
        npmCli,
        "install",
        "--offline",
        "--ignore-scripts",
        "--no-audit",
        "--no-fund",
        "--package-lock=false",
        path.join(outputDirectory, artifactName),
      ],
      { cwd: consumerDirectory, env: npmEnvironment, timeout: 60_000 },
    );
    if (artifactName.includes("local-mcp-connector")) {
      const installedPackageRoot = path.join(
        consumerDirectory,
        "node_modules",
        "@agentpay",
        "local-mcp-connector",
      );
      await assertNoPackedSecrets(installedPackageRoot);
      const connectorPackage = JSON.parse(
        await readFile(path.join(installedPackageRoot, "package.json"), "utf8"),
      );
      assert.deepEqual(connectorPackage.bin, {
        "agentpay-mcp": "dist/cli.js",
      });
      const connectorEntry = path.join(installedPackageRoot, "dist", "cli.js");
      await assert.rejects(
        execFileAsync(process.execPath, [connectorEntry, "--check"], {
          cwd: consumerDirectory,
          env: {
            ...process.env,
            AGENTPAY_API_BASE_URL: "https://api.agentpay.test",
            AGENTPAY_PROJECT_KEY: "",
          },
          timeout: 10_000,
        }),
        (error) => {
          assert.equal(error.code, 1);
          assert.match(error.stderr, /AGENTPAY_PROJECT_KEY is required/u);
          return true;
        },
      );
    } else {
      const installedPackageRoot = path.join(
        consumerDirectory,
        "node_modules",
        "@agentpay",
        "merchant-sdk",
      );
      await assertNoPackedSecrets(installedPackageRoot);
      const sdkEntry = path.join(installedPackageRoot, "dist", "index.js");
      const sdk = await import(
        `${pathToFileURL(sdkEntry).href}?test=${Date.now()}`
      );
      assert.equal(typeof sdk.verifyExecutionRequest, "function");
      assert.equal(typeof sdk.verifyAgentPayWebhook, "function");
      assert.equal(
        await stat(path.join(installedPackageRoot, "dist", "dynamodb.js")).then(
          (entry) => entry.isFile(),
        ),
        true,
      );
      const sdkPackage = JSON.parse(
        await readFile(path.join(installedPackageRoot, "package.json"), "utf8"),
      );
      assert.deepEqual(sdkPackage.bundleDependencies, [
        "@agentpay/verify-node",
      ]);
      assert.equal(
        await stat(
          path.join(
            installedPackageRoot,
            "node_modules",
            "@agentpay",
            "verify-node",
            "dist",
            "index.d.ts",
          ),
        ).then((entry) => entry.isFile()),
        true,
      );
    }
  }
}

async function assertNoPackedSecrets(rootDirectory) {
  const forbiddenContent = [
    /-----BEGIN (?:EC |RSA |OPENSSH )?PRIVATE KEY-----/u,
    /\bAKIA[0-9A-Z]{16}\b/u,
    /\bghp_[A-Za-z0-9]{20,}\b/u,
    /\bsk_live_[A-Za-z0-9]{16,}\b/u,
    /AGENTPAY_PROJECT_KEY\s*=\s*apc[12]\./u,
  ];
  for (const filePath of await listFiles(rootDirectory)) {
    if (isForbiddenPackedPath(path.relative(rootDirectory, filePath))) {
      assert.fail(`forbidden packaged path: ${filePath}`);
    }
    const fileStat = await stat(filePath);
    if (fileStat.size > 2 * 1024 * 1024) continue;
    const content = await readFile(filePath, "utf8");
    for (const pattern of forbiddenContent) {
      assert.equal(
        pattern.test(content),
        false,
        `secret pattern in ${filePath}`,
      );
    }
  }
}

function isForbiddenPackedPath(filePath) {
  return /(?:^|[\\/])(?:test|tests|\.cache)(?:[\\/]|$)|\.map$|(?:^|[\\/])\.env(?:\.|$)/u.test(
    filePath,
  );
}

async function listFiles(rootDirectory) {
  const entries = await readdir(rootDirectory, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const entryPath = path.join(rootDirectory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await listFiles(entryPath)));
    } else if (entry.isFile()) {
      files.push(entryPath);
    }
  }
  return files;
}

async function sha256(filePath) {
  return createHash("sha256")
    .update(await readFile(filePath))
    .digest("hex");
}
