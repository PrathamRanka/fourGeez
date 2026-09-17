import { spawnSync } from "node:child_process";

const unittestArguments = [
  "-m",
  "unittest",
  "discover",
  "-s",
  "verification/python/tests",
  "-t",
  "verification/python",
  "-v",
];

const configuredPython = process.env.AGENTPAY_PYTHON
  ? [{ command: process.env.AGENTPAY_PYTHON, arguments: unittestArguments }]
  : [];

const candidates = process.platform === "win32"
  ? [
      ...configuredPython,
      {
        command: "powershell.exe",
        arguments: [
          "-NoProfile",
          "-ExecutionPolicy",
          "Bypass",
          "-File",
          "scripts/run-python-verification-tests.ps1",
        ],
      },
      { command: "python", arguments: unittestArguments },
    ]
  : [
      ...configuredPython,
      { command: "python3", arguments: unittestArguments },
      { command: "python", arguments: unittestArguments },
    ];

// runCandidate executes one Python command without invoking a shell directly.
function runCandidate(candidate) {
  return spawnSync(candidate.command, candidate.arguments, {
    cwd: process.cwd(),
    env: process.env,
    stdio: "inherit",
  });
}

for (const candidate of candidates) {
  const result = runCandidate(candidate);
  if (!result.error) {
    process.exit(result.status ?? 1);
  }
}

console.error("Python 3.11 or newer is required to run verification tests.");
process.exit(1);
