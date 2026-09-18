import { spawn } from "node:child_process";
import { resolve } from "node:path";
import { expect, test } from "./fixtures/agentpay";

test("launch pages meet Lighthouse accessibility and best-practice budgets", async ({
  seed,
}) => {
  test.setTimeout(120_000);
  void seed;
  const result = await new Promise<{ code: number | null; output: string }>(
    (resolveResult) => {
      const child = spawn(
        process.execPath,
        [resolve("scripts/lighthouse-budget.mjs")],
        {
          cwd: process.cwd(),
          env: process.env,
          stdio: ["ignore", "pipe", "pipe"],
        },
      );
      let output = "";
      child.stdout.on("data", (chunk) => (output += String(chunk)));
      child.stderr.on("data", (chunk) => (output += String(chunk)));
      child.on("close", (code) => resolveResult({ code, output }));
    },
  );
  expect(result.code, result.output).toBe(0);
});
