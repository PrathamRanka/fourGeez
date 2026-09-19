import { createHash } from "node:crypto";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";

const expectedPackages = new Set([
  "@agentpay/local-mcp-connector",
  "@agentpay/merchant-sdk",
]);
const releaseDirectory = parseDirectory(process.argv.slice(2));
await verifyRelease(releaseDirectory);

async function verifyRelease(directory) {
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
    !Array.isArray(provenance.artifacts) ||
    provenance.artifacts.length !== 2
  ) {
    throw new Error("Package provenance must describe exactly two artifacts");
  }
  const checksumManifest = parseChecksums(
    await readFile(path.join(directory, "SHA256SUMS"), "utf8"),
  );
  const packageNames = new Set();
  for (const artifact of provenance.artifacts) {
    packageNames.add(artifact.packageName);
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

function parseDirectory(argumentsList) {
  if (argumentsList.length !== 2 || argumentsList[0] !== "--directory") {
    throw new Error("Usage: npm run package:verify -- --directory <directory>");
  }
  return path.resolve(argumentsList[1]);
}
