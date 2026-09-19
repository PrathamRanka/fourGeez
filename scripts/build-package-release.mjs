import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { promisify } from "node:util";

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
  npm_config_cache: path.join(repositoryRoot, ".cache", "npm-package-release"),
  npm_config_update_notifier: "false",
};
const releasePackages = [
  {
    workspace: "@agentpay/local-mcp-connector",
    directory: "packages/local-mcp-connector",
    buildWorkspaces: ["@agentpay/local-mcp-connector"],
    requiredFiles: [
      "LICENSE",
      "NOTICE",
      "README.md",
      "dist/cli.js",
      "package.json",
    ],
  },
  {
    workspace: "@agentpay/merchant-sdk",
    directory: "packages/merchant-sdk",
    buildWorkspaces: ["@agentpay/verify-node", "@agentpay/merchant-sdk"],
    requiredFiles: [
      "LICENSE",
      "NOTICE",
      "README.md",
      "dist/index.d.ts",
      "dist/index.js",
      "node_modules/@agentpay/verify-node/dist/index.js",
      "package.json",
    ],
  },
];

const { outputDirectory, allowDirty } = parseArguments(process.argv.slice(2));
await buildRelease(outputDirectory, allowDirty);

async function buildRelease(outputDirectory, allowDirty) {
  const commit = await gitOutput(["rev-parse", "HEAD"]);
  const dirty =
    (await gitOutput(["status", "--porcelain", "--untracked-files=all"])) !==
    "";
  if (dirty && !allowDirty) {
    throw new Error(
      "Package releases require a clean worktree. Commit the reviewed change before building artifacts.",
    );
  }

  await rm(outputDirectory, { recursive: true, force: true });
  await mkdir(outputDirectory, { recursive: true });

  const artifacts = [];
  for (const releasePackage of releasePackages) {
    for (const workspace of releasePackage.buildWorkspaces) {
      await runNpm(["run", "build", "--workspace", workspace]);
    }
    const stagingRoot = await mkdtemp(
      path.join(tmpdir(), "agentpay-package-stage-"),
    );
    try {
      const packageStage = path.join(stagingRoot, "package");
      await stagePackage(releasePackage, packageStage);
      const dryRun = await npmJson([
        "pack",
        "--dry-run",
        "--json",
        packageStage,
      ]);
      validatePackReport(releasePackage, packageStage, dryRun[0]);
      const packed = await npmJson([
        "pack",
        "--json",
        "--pack-destination",
        outputDirectory,
        packageStage,
      ]);
      const report = packed[0];
      validatePackReport(releasePackage, packageStage, report);
      const artifactPath = path.join(outputDirectory, report.filename);
      artifacts.push({
        packageName: report.name,
        version: report.version,
        fileName: report.filename,
        sha256: await sha256(artifactPath),
        npmIntegrity: report.integrity,
        unpackedSize: report.unpackedSize,
        packedFiles: report.files.map((entry) => entry.path).sort(),
      });
    } finally {
      await rm(stagingRoot, { recursive: true, force: true });
    }
  }

  artifacts.sort((left, right) => left.fileName.localeCompare(right.fileName));
  const versions = new Set(artifacts.map((artifact) => artifact.version));
  if (versions.size !== 1) {
    throw new Error(
      "Seller package versions must remain aligned for a release",
    );
  }
  const [version] = versions;
  const provenance = {
    schemaVersion: "agentpay.package-release.v1",
    releaseId: `agentpay-packages-v${version}`,
    source: {
      repository: "https://github.com/PrathamRanka/fourGeez",
      commit,
      dirty,
    },
    toolchain: {
      node: process.version,
      npm: await npmOutput(["--version"]),
    },
    artifacts,
  };
  await writeFile(
    path.join(outputDirectory, "provenance.json"),
    `${JSON.stringify(provenance, null, 2)}\n`,
  );
  await writeFile(
    path.join(outputDirectory, "SHA256SUMS"),
    `${artifacts.map((artifact) => `${artifact.sha256}  ${artifact.fileName}`).join("\n")}\n`,
  );
  process.stdout.write(
    `Created ${artifacts.length} verified seller package artifacts in ${outputDirectory}\n`,
  );
}

async function stagePackage(releasePackage, packageStage) {
  const packageDirectory = path.join(repositoryRoot, releasePackage.directory);
  await mkdir(packageStage, { recursive: true });
  for (const entry of [
    "package.json",
    "README.md",
    "LICENSE",
    "NOTICE",
    "dist",
  ]) {
    await cp(
      path.join(packageDirectory, entry),
      path.join(packageStage, entry),
      {
        recursive: true,
      },
    );
  }
  if (releasePackage.workspace === "@agentpay/merchant-sdk") {
    const verifierSource = path.join(repositoryRoot, "verification", "node");
    const verifierStage = path.join(
      packageStage,
      "node_modules",
      "@agentpay",
      "verify-node",
    );
    await mkdir(verifierStage, { recursive: true });
    for (const entry of ["package.json", "LICENSE", "NOTICE", "dist"]) {
      await cp(
        path.join(verifierSource, entry),
        path.join(verifierStage, entry),
        {
          recursive: true,
        },
      );
    }
  }
}

function validatePackReport(releasePackage, packageDirectory, report) {
  if (!report || report.name !== releasePackage.workspace) {
    throw new Error(
      `Unexpected npm pack report for ${releasePackage.workspace}`,
    );
  }
  const manifest = JSON.parse(
    requireTextFile(path.join(packageDirectory, "package.json")),
  );
  if (manifest.private !== true) {
    throw new Error(`${releasePackage.workspace} must remain private`);
  }
  if (manifest.license !== "SEE LICENSE IN LICENSE") {
    throw new Error(
      `${releasePackage.workspace} must reference its packed license`,
    );
  }
  for (const lifecycleName of [
    "preinstall",
    "install",
    "postinstall",
    "prepare",
  ]) {
    if (manifest.scripts?.[lifecycleName]) {
      throw new Error(
        `${releasePackage.workspace} cannot ship ${lifecycleName}`,
      );
    }
  }
  const packedFiles = report.files.map((entry) => entry.path);
  for (const requiredFile of releasePackage.requiredFiles) {
    if (!packedFiles.includes(requiredFile)) {
      throw new Error(
        `${releasePackage.workspace} is missing packed file ${requiredFile}`,
      );
    }
  }
  const forbiddenFile = packedFiles.find((filePath) =>
    /(?:^|\/)(?:test|tests|\.cache)(?:\/|$)|\.map$|(?:^|\/)\.env(?:\.|$)|package-lock\.json$/u.test(
      filePath,
    ),
  );
  if (forbiddenFile) {
    throw new Error(
      `${releasePackage.workspace} includes forbidden file ${forbiddenFile}`,
    );
  }
  scanPackSources(packageDirectory, packedFiles);
}

function scanPackSources(packageDirectory, packedFiles) {
  const forbiddenPatterns = [
    /-----BEGIN (?:EC |RSA |OPENSSH )?PRIVATE KEY-----/u,
    /\bAKIA[0-9A-Z]{16}\b/u,
    /\bghp_[A-Za-z0-9]{20,}\b/u,
    /\bsk_live_[A-Za-z0-9]{16,}\b/u,
    /AGENTPAY_PROJECT_KEY\s*=\s*apc[12]\./u,
  ];
  for (const packedFile of packedFiles) {
    const sourcePath = path.join(packageDirectory, packedFile);
    let content;
    try {
      content = requireTextFile(sourcePath);
    } catch {
      continue;
    }
    for (const pattern of forbiddenPatterns) {
      if (pattern.test(content)) {
        throw new Error(`Potential secret material found in ${packedFile}`);
      }
    }
  }
}

function requireTextFile(filePath) {
  return readFileSync(filePath, "utf8");
}

async function npmJson(argumentsList) {
  return JSON.parse(await npmOutput(argumentsList));
}

async function runNpm(argumentsList) {
  await execFileAsync(process.execPath, [npmCli, ...argumentsList], {
    cwd: repositoryRoot,
    env: npmEnvironment,
    timeout: 120_000,
    maxBuffer: 16 * 1024 * 1024,
  });
}

async function npmOutput(argumentsList) {
  const { stdout } = await execFileAsync(
    process.execPath,
    [npmCli, ...argumentsList],
    {
      cwd: repositoryRoot,
      env: npmEnvironment,
      timeout: 120_000,
      maxBuffer: 16 * 1024 * 1024,
    },
  );
  return stdout.trim();
}

async function gitOutput(argumentsList) {
  const { stdout } = await execFileAsync("git", argumentsList, {
    cwd: repositoryRoot,
    timeout: 30_000,
  });
  return stdout.trim();
}

async function sha256(filePath) {
  return createHash("sha256")
    .update(await readFile(filePath))
    .digest("hex");
}

function parseArguments(argumentsList) {
  let outputDirectory;
  let allowDirty = false;
  for (let index = 0; index < argumentsList.length; index += 1) {
    const argument = argumentsList[index];
    if (argument === "--output") {
      const value = argumentsList[index + 1];
      if (!value) throw new Error("--output requires a directory");
      outputDirectory = path.resolve(repositoryRoot, value);
      index += 1;
    } else if (argument === "--allow-dirty") {
      allowDirty = true;
    } else {
      throw new Error(`Unknown argument: ${argument}`);
    }
  }
  if (!outputDirectory) {
    throw new Error("Usage: npm run package:release -- --output <directory>");
  }
  return { outputDirectory, allowDirty };
}
