import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  collectDescendantProcessIDs,
  expandKnownProcessIDs,
  createCleanupGuardianArguments,
  createLocalRuntimeConfig,
  installShutdownHandlers,
  runLocalRuntime,
  shutdownChildren,
  validateLocalRuntimeConfig,
  waitForServices,
} from "./dev-local.mjs";

const deterministicRandomBytes = (size) => Buffer.alloc(size, 7);

test("local runtime config starts every production-shaped dependency", () => {
  const config = createLocalRuntimeConfig({}, {
    platform: "win32",
    randomBytes: deterministicRandomBytes,
  });

  assert.deepEqual(
    config.processes.map(({ name }) => name),
    ["facilitator", "seller", "api", "web"],
  );
  assert.equal(config.repositoryMode, "memory");
  assert.equal(config.buyerMode, "deterministic");
  assert.equal(config.processes[2].env.AGENTPAY_USE_MOCK_PAYMENT, "true");
  assert.equal(config.processes[2].env.AGENTPAY_LOCAL_SEED_PROFILE, "launch-ready");
  assert.equal(
    config.processes[2].env.AGENTPAY_PAYMENT_READINESS_URL,
    "http://127.0.0.1:8091/verify",
  );
  assert.equal(
    config.processes[2].env.AGENTPAY_SELLER_READINESS_URL,
    "http://127.0.0.1:8090/research/basic",
  );
  assert.equal(
    config.processes[3].env.AGENTPAY_DEMO_SELLER_ID,
    "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
  );
  assert.deepEqual(config.processes[2].args, [
    "run",
    "-tags",
    "agentpay_dev",
    "./cmd/api",
  ]);
  for (const processSpecification of config.processes.slice(0, 3)) {
    assert.match(processSpecification.env.GOCACHE, /\.cache[\\/]go-build$/i);
  }
  assert.equal(config.processes[2].env.AGENTPAY_WEB_ORIGIN, "http://localhost:3000");
  assert.equal(config.processes[3].env.AGENTPAY_WS_ORIGIN, "ws://127.0.0.1:8080");
  assert.equal(
    config.processes[3].env.AGENTPAY_SELLER_BEARER_TOKEN,
    config.processes[2].env.AGENTPAY_LOCAL_SELLER_TOKEN,
  );
  assert.equal(config.processes[3].command, process.execPath);
  assert.match(config.processes[3].args[0], /npm-cli\.js$/i);
  assert.deepEqual(
    config.services.map(({ name, url, method }) => ({ name, url, method })),
    [
      {
        name: "facilitator",
        url: "http://127.0.0.1:8091/verify",
        method: "POST",
      },
      {
        name: "seller",
        url: "http://127.0.0.1:8090/research/basic",
        method: "POST",
      },
      { name: "api", url: "http://127.0.0.1:8080/health/ready", method: "GET" },
      { name: "web", url: "http://localhost:3000/", method: "GET" },
    ],
  );
});

test("configuration validation rejects conflicting ports and non-loopback origins", () => {
  assert.throws(
    () => createLocalRuntimeConfig({ AGENTPAY_LOCAL_API_PORT: "8090" }),
    /port 8090 is assigned to both api and seller/i,
  );

  const config = createLocalRuntimeConfig({}, {
    platform: "linux",
    randomBytes: deterministicRandomBytes,
  });
  config.webOrigin = "https://example.com";
  assert.throws(
    () => validateLocalRuntimeConfig(config),
    /web origin must use localhost or a loopback address/i,
  );
});

test("waitForServices retries dependencies and reports the dependency that timed out", async () => {
  const attempts = new Map();
  const config = createLocalRuntimeConfig({}, {
    platform: "linux",
    randomBytes: deterministicRandomBytes,
  });

  await waitForServices(config.services.slice(0, 2), {
    readinessTimeoutMilliseconds: 100,
    retryDelayMilliseconds: 1,
    probe: async (service) => {
      const attempt = (attempts.get(service.name) ?? 0) + 1;
      attempts.set(service.name, attempt);
      return attempt >= 2;
    },
  });
  assert.equal(attempts.get("facilitator"), 2);
  assert.equal(attempts.get("seller"), 2);

  await assert.rejects(
    waitForServices([config.services[2]], {
      readinessTimeoutMilliseconds: 5,
      retryDelayMilliseconds: 1,
      probe: async () => false,
    }),
    /api failed readiness at http:\/\/127\.0\.0\.1:8080\/health/i,
  );
});

test("the first Ctrl+C shuts down each process tree exactly once", async () => {
  const signalSource = new EventEmitter();
  const children = [
    { name: "api", process: { pid: 101, exitCode: null } },
    { name: "web", process: { pid: 102, exitCode: null } },
  ];
  const terminated = [];
  let resolveCompletion;
  const completion = new Promise((resolve) => {
    resolveCompletion = resolve;
  });

  installShutdownHandlers({
    signalSource,
    shutdown: async (reason) => {
      await shutdownChildren(children, {
        platform: "win32",
        terminateTree: async (pid) => terminated.push(pid),
      });
      resolveCompletion(reason);
    },
  });

  signalSource.emit("SIGINT");
  signalSource.emit("SIGINT");

  assert.equal(await completion, "SIGINT");
  assert.deepEqual(terminated, [101, 102]);
});

test("shutdown still cleans descendants after a launcher exits and tolerates missing trees", async () => {
  const terminated = [];

  await shutdownChildren(
    [
      { name: "exited", process: { pid: 201, exitCode: 0 } },
      { name: "running", process: { pid: 202, exitCode: null } },
      { name: "unstarted", process: { pid: undefined, exitCode: null } },
    ],
    {
      platform: "win32",
      terminateTree: async (pid) => {
        terminated.push(pid);
        const error = new Error("tree already gone");
        error.code = "ESRCH";
        throw error;
      },
    },
  );

  assert.deepEqual(terminated, [201, 202]);
});

test("Windows shutdown snapshots descendants and orders them before their launcher", () => {
  const processIDs = collectDescendantProcessIDs(
    [
      { processId: 100, parentProcessId: 50 },
      { processId: 101, parentProcessId: 100 },
      { processId: 102, parentProcessId: 101 },
      { processId: 103, parentProcessId: 100 },
      { processId: 999, parentProcessId: 50 },
    ],
    100,
  );

  assert.deepEqual(processIDs, [102, 101, 103, 100]);
});

test("the detached cleanup guardian receives only launcher and service process IDs", () => {
  assert.deepEqual(
    createCleanupGuardianArguments(50, [
      { name: "api", process: { pid: 101 } },
      { name: "web", process: { pid: 102 } },
    ]),
    ["--cleanup-guardian", "50", "101", "102"],
  );
  assert.throws(
    () => createCleanupGuardianArguments(0, []),
    /launcher process ID must be a positive integer/i,
  );
});

test("the cleanup guardian retains descendants after intermediate launchers exit", () => {
  const firstSnapshot = expandKnownProcessIDs(
    [100],
    [
      { processId: 101, parentProcessId: 100 },
      { processId: 102, parentProcessId: 101 },
    ],
  );
  const secondSnapshot = expandKnownProcessIDs(firstSnapshot, [
    { processId: 103, parentProcessId: 102 },
  ]);

  assert.deepEqual(firstSnapshot, [100, 101, 102]);
  assert.deepEqual(secondSnapshot, [100, 101, 102, 103]);
});

test("the repository exposes one documented local-runtime command", async () => {
  const packageDocument = JSON.parse(await readFile(new URL("../package.json", import.meta.url)));

  assert.equal(packageDocument.scripts["dev:local"], "node scripts/dev-local.mjs");
  assert.equal(
    packageDocument.scripts["test:dev-local"],
    "node --test scripts/dev-local.test.mjs",
  );
});

test("runLocalRuntime treats Ctrl+C as a clean supervised shutdown", async () => {
  const signalSource = new EventEmitter();
  const childProcess = Object.assign(new EventEmitter(), { pid: 301, exitCode: null });
  const children = [{ name: "api", process: childProcess }];
  const outputLines = [];
  let shutdownCalls = 0;
  let guardianCalls = 0;
  const config = createLocalRuntimeConfig({}, {
    platform: "win32",
    randomBytes: deterministicRandomBytes,
  });

  const resultPromise = runLocalRuntime(config, {
    output: { write: (value) => outputLines.push(value) },
    errorOutput: { write: (value) => outputLines.push(value) },
    signalSource,
    validatePrerequisites: async () => {},
    spawnProcesses: () => children,
    startCleanupGuardian: () => {
      guardianCalls += 1;
    },
    waitForServices: async () => {},
    shutdownChildren: async () => {
      shutdownCalls += 1;
      childProcess.exitCode = 0;
      childProcess.emit("exit", 0, null);
    },
  });

  await new Promise((resolve) => setImmediate(resolve));
  signalSource.emit("SIGINT");

  assert.equal(await resultPromise, 0);
  assert.equal(shutdownCalls, 1);
  assert.equal(guardianCalls, 1);
  assert.match(outputLines.join(""), /local runtime is ready/i);
});

test("runLocalRuntime surfaces an unexpected dependency exit", async () => {
  const signalSource = new EventEmitter();
  const childProcess = Object.assign(new EventEmitter(), { pid: 401, exitCode: null });
  const errors = [];
  const config = createLocalRuntimeConfig({}, {
    platform: "linux",
    randomBytes: deterministicRandomBytes,
  });

  const resultPromise = runLocalRuntime(config, {
    output: { write: () => {} },
    errorOutput: { write: (value) => errors.push(value) },
    signalSource,
    validatePrerequisites: async () => {},
    spawnProcesses: () => [{ name: "seller", process: childProcess }],
    waitForServices: () => new Promise(() => {}),
    shutdownChildren: async () => {
      childProcess.exitCode = 2;
    },
  });

  await new Promise((resolve) => setImmediate(resolve));
  childProcess.exitCode = 2;
  childProcess.emit("exit", 2, null);

  assert.equal(await resultPromise, 1);
  assert.match(errors.join(""), /seller exited unexpectedly \(code 2\)/i);
});

test("Ctrl+C during preflight never starts a late process graph", async () => {
  const signalSource = new EventEmitter();
  let resolvePreflight;
  const preflight = new Promise((resolve) => {
    resolvePreflight = resolve;
  });
  let spawnCalls = 0;
  const config = createLocalRuntimeConfig({}, {
    platform: "linux",
    randomBytes: deterministicRandomBytes,
  });

  const resultPromise = runLocalRuntime(config, {
    output: { write: () => {} },
    errorOutput: { write: () => {} },
    signalSource,
    validatePrerequisites: () => preflight,
    spawnProcesses: () => {
      spawnCalls += 1;
      return [];
    },
    waitForServices: async () => {},
    shutdownChildren: async () => {},
  });

  signalSource.emit("SIGINT");
  resolvePreflight();

  assert.equal(await resultPromise, 0);
  assert.equal(spawnCalls, 0);
});
