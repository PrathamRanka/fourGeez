import { spawnSync } from "node:child_process";
import { mkdirSync, rmSync } from "node:fs";
import path from "node:path";

const repositoryRoot = path.resolve(import.meta.dirname, "..");
const outputDirectory = path.join(repositoryRoot, "infra", "terraform", "dist");
const bootstrapPath = path.join(outputDirectory, "bootstrap");
const archivePath = path.join(outputDirectory, "agentpay-api.zip");

mkdirSync(outputDirectory, { recursive: true });
rmSync(bootstrapPath, { force: true });
rmSync(archivePath, { force: true });

run(
  "go",
  ["build", "-trimpath", "-ldflags", "-s -w", "-o", bootstrapPath, "./cmd/api"],
  {
    ...process.env,
    CGO_ENABLED: "0",
    GOOS: "linux",
    GOARCH: "arm64",
  },
);
run(
  "go",
  ["run", "./scripts/package-lambda.go", bootstrapPath, archivePath],
  process.env,
);

function run(command, args, env) {
  const result = spawnSync(command, args, {
    cwd: repositoryRoot,
    env,
    stdio: "inherit",
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
