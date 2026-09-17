import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";

const cache = resolve(".cache", "go-build");
mkdirSync(cache, { recursive: true });

const result = spawnSync("go", process.argv.slice(2), {
  env: { ...process.env, GOCACHE: cache },
  shell: process.platform === "win32",
  stdio: "inherit",
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 1);
