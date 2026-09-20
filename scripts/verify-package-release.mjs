import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";

const expectedPackages = new Set([
  "@agentpay/local-mcp-connector",
  "@agentpay/merchant-sdk",
]);
const options = parseArguments(process.argv.slice(2));
await verifyRelease(options);

async function verifyRelease({
  directory,
  expectedTag,
  expectedCommit,
  allowDirty,
}) {
  const provenance = JSON.parse(
    await readFile(path.join(directory, "provenance.json"), "utf8"),
  );
  if (provenance.schemaVersion !== "agentpay.package-release.v1") {
    throw new Error(
      "Unsupported or missing AgentPay package provenance schema",
    );
  }
  if (!/^[a-f0-9]{40}$/u.test(provenance.source?.commit ?? "")) {
    throw new Error("Package provenance does not contain a full source commit");
  }
  if (
    provenance.source.repository !==
    "https://github.com/PrathamRanka/fourGeez"
  ) {
    throw new Error("Package provenance contains an unexpected source repository");
  }
  if (provenance.source.dirty !== false && !allowDirty) {
    throw new Error("Package provenance records a dirty source worktree");
  }
  if (expectedCommit && provenance.source.commit !== expectedCommit) {
    throw new Error("source commit does not match provenance");
  }
  if (
    !Array.isArray(provenance.artifacts) ||
    provenance.artifacts.length !== 2
  ) {
    throw new Error("Package provenance must describe exactly two artifacts");
  }
  const checksumManifest = parseChecksums(
    await readFile(path.join(directory, "SHA256SUMS"), "utf8"),
  );
  const packageNames = new Set();
  const versions = new Set();
  for (const artifact of provenance.artifacts) {
    packageNames.add(artifact.packageName);
    versions.add(artifact.version);
    if (!/^[a-z0-9@/._-]+\.tgz$/u.test(artifact.fileName)) {
      throw new Error("Package provenance contains an invalid artifact name");
    }
    if (!/^[a-f0-9]{64}$/u.test(artifact.sha256)) {
      throw new Error(`Invalid SHA-256 for ${artifact.fileName}`);
    }
    const actual = await sha256(path.join(directory, artifact.fileName));
    if (
      actual !== artifact.sha256 ||
      checksumManifest.get(artifact.fileName) !== actual
    ) {
      throw new Error(`Checksum verification failed for ${artifact.fileName}`);
    }
  }
  if (versions.size !== 1) {
    throw new Error("Package provenance contains mismatched versions");
  }
  const [version] = versions;
  const releaseTag = `agentpay-packages-v${version}`;
  if (provenance.releaseId !== releaseTag) {
    throw new Error("release ID does not match package versions");
  }
  if (expectedTag && releaseTag !== expectedTag) {
    throw new Error("release tag does not match provenance");
  }
  assertEqualSets(packageNames, expectedPackages, "unexpected package set");
  assertEqualSets(
    new Set(checksumManifest.keys()),
    new Set(provenance.artifacts.map((artifact) => artifact.fileName)),
    "checksum manifest does not match provenance",
  );
  const tarballs = (await readdir(directory)).filter((name) =>
    name.endsWith(".tgz"),
  );
  assertEqualSets(
    new Set(tarballs),
    new Set(provenance.artifacts.map((artifact) => artifact.fileName)),
    "release directory contains an unexpected tarball",
  );
  process.stdout.write(
    `Verified ${provenance.artifacts.length} AgentPay package artifacts from ${provenance.source.commit}\n`,
  );
}

function parseChecksums(content) {
  const checksums = new Map();
  for (const line of content.trim().split(/\r?\n/u)) {
    const match = /^([a-f0-9]{64})  ([A-Za-z0-9@._/-]+\.tgz)$/u.exec(line);
    if (!match || checksums.has(match[2])) {
      throw new Error("Invalid SHA256SUMS manifest");
    }
    checksums.set(match[2], match[1]);
  }
  return checksums;
}

function assertEqualSets(actual, expected, message) {
  if (
    actual.size !== expected.size ||
    [...actual].some((value) => !expected.has(value))
  ) {
    throw new Error(message);
  }
}

async function sha256(filePath) {
  return createHash("sha256")
    .update(await readFile(filePath))
    .digest("hex");
}

function parseArguments(argumentsList) {
  const options = {
    directory: "",
    expectedTag: "",
    expectedCommit: "",
    allowDirty: false,
  };
  for (let index = 0; index < argumentsList.length; index += 1) {
    const argument = argumentsList[index];
    if (argument === "--allow-dirty") {
      options.allowDirty = true;
      continue;
    }
    if (
      argument === "--directory" ||
      argument === "--expected-tag" ||
      argument === "--expected-commit"
    ) {
      const value = argumentsList[index + 1];
      if (!value) throw new Error(`${argument} requires a value`);
      if (argument === "--directory") options.directory = path.resolve(value);
      if (argument === "--expected-tag") options.expectedTag = value;
      if (argument === "--expected-commit") options.expectedCommit = value;
      index += 1;
      continue;
    }
    throw new Error(`Unknown argument: ${argument}`);
  }
  if (!options.directory) {
    throw new Error(
      "Usage: npm run package:verify -- --directory <directory> [--expected-tag <tag>] [--expected-commit <sha>]",
    );
  }
  if (
    options.expectedTag &&
    !/^agentpay-packages-v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/u.test(
      options.expectedTag,
    )
  ) {
    throw new Error("expected release tag is invalid");
  }
  if (
    options.expectedCommit &&
    !/^[a-f0-9]{40}$/u.test(options.expectedCommit)
  ) {
    throw new Error("expected source commit must be a full Git SHA");
  }
  return options;
}
