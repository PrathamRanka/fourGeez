import { randomBytes as cryptoRandomBytes } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { createServer } from "node:net";
import { dirname, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const defaultPorts = Object.freeze({
  web: 3000,
  api: 8080,
  seller: 8090,
  facilitator: 8091,
});
const defaultReadinessTimeoutMilliseconds = 60_000;
const defaultRetryDelayMilliseconds = 250;
const guardianSnapshotDelaysMilliseconds = [0, 1_000, 2_000];
const guardianParentPollMilliseconds = 250;
const localSecretBytes = 32;
const launchReadySellerID = "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7";

function parsePort(rawValue, name, fallback) {
  const value = rawValue === undefined || rawValue === "" ? fallback : Number(rawValue);
  if (!Number.isInteger(value) || value < 1 || value > 65_535) {
    throw new Error(`${name} port must be an integer from 1 through 65535.`);
  }
  return value;
}

function createSecret(prefix, randomBytes) {
  return `${prefix}_${randomBytes(localSecretBytes).toString("base64url")}`;
}

function localURL(name, protocol, host, port, path = "") {
  return { name, url: `${protocol}://${host}:${port}${path}` };
}

// createLocalRuntimeConfig creates ephemeral credentials and one shared process plan.
export function createLocalRuntimeConfig(environment = process.env, options = {}) {
  const platform = options.platform ?? process.platform;
  const randomBytes = options.randomBytes ?? cryptoRandomBytes;
  const ports = {
    web: parsePort(environment.AGENTPAY_LOCAL_WEB_PORT, "web", defaultPorts.web),
    api: parsePort(environment.AGENTPAY_LOCAL_API_PORT, "api", defaultPorts.api),
    seller: defaultPorts.seller,
    facilitator: defaultPorts.facilitator,
  };
  const webOrigin = `http://localhost:${ports.web}`;
  const apiOrigin = `http://127.0.0.1:${ports.api}`;
  const sellerToken = createSecret("local_seller", randomBytes);
  const agentKey = createSecret("local_agent", randomBytes);
  const goBuildCache = resolve(repositoryRoot, ".cache", "go-build");
  const apiEnvironment = {
    AGENTPAY_ENV: "local",
    AGENTPAY_HTTP_ADDR: `127.0.0.1:${ports.api}`,
    AGENTPAY_PUBLIC_BASE_URL: apiOrigin,
    AGENTPAY_WEB_ORIGIN: webOrigin,
    AGENTPAY_REPOSITORY_MODE: "memory",
    AGENTPAY_PAYMENT_MODE: "mock",
    AGENTPAY_USE_MOCK_PAYMENT: "true",
    AGENTPAY_MOCK_FACILITATOR_URL: `http://127.0.0.1:${ports.facilitator}`,
    AGENTPAY_PAYMENT_READINESS_URL: `http://127.0.0.1:${ports.facilitator}/verify`,
    AGENTPAY_SELLER_READINESS_URL: `http://127.0.0.1:${ports.seller}/research/basic`,
    AGENTPAY_BEDROCK_MODE: "disabled",
    AGENTPAY_BUYER_MODE: "deterministic",
    AGENTPAY_LOCAL_SELLER_TOKEN: sellerToken,
    AGENTPAY_LOCAL_AGENT_KEY: agentKey,
    AGENTPAY_LOCAL_SELLER_SIGNING_SECRET: createSecret("seller_signing", randomBytes),
    AGENTPAY_LOCAL_APPROVAL_TOKEN_SECRET: createSecret("approval_signing", randomBytes),
    AGENTPAY_LOCAL_EVIDENCE_KEY_ID: "local-evidence-key-v1",
    AGENTPAY_LOCAL_EVIDENCE_SIGNING_SECRET: createSecret("evidence_signing", randomBytes),
    AGENTPAY_LOCAL_WEBHOOK_SIGNING_SECRET: createSecret("webhook_signing", randomBytes),
    AGENTPAY_LOCAL_SEED_PROFILE: "launch-ready",
    GOCACHE: goBuildCache,
  };
  const webEnvironment = {
    AGENTPAY_ENV: "local",
    AGENTPAY_API_ORIGIN: apiOrigin,
    AGENTPAY_WEB_ORIGIN: webOrigin,
    AGENTPAY_WS_ORIGIN: `ws://127.0.0.1:${ports.api}`,
    AGENTPAY_SELLER_BEARER_TOKEN: sellerToken,
    AGENTPAY_DEMO_SELLER_ID: launchReadySellerID,
    AGENTPAY_BEDROCK_MODE: "disabled",
    AGENTPAY_BUYER_MODE: "deterministic",
    NEXT_TELEMETRY_DISABLED: "1",
  };
  const npmCLIPath =
    environment.npm_execpath ??
    process.env.npm_execpath ??
    resolve(dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js");
  const npmCommand = platform === "win32" ? process.execPath : "npm";
  const npmArgumentsPrefix = platform === "win32" ? [npmCLIPath] : [];
  const processes = [
    {
      name: "facilitator",
      command: "go",
      args: ["run", "./examples/mock-facilitator"],
      env: { AGENTPAY_ENV: "local", GOCACHE: goBuildCache },
    },
    {
      name: "seller",
      command: "go",
      args: ["run", "./examples/demo-seller"],
      env: { AGENTPAY_ENV: "local", GOCACHE: goBuildCache },
    },
    {
      name: "api",
      command: "go",
      args: ["run", "-tags", "agentpay_dev", "./cmd/api"],
      env: apiEnvironment,
    },
    {
      name: "web",
      command: npmCommand,
      args: [
        ...npmArgumentsPrefix,
        "run",
        "dev",
        "--workspace",
        "@agentpay/web",
        "--",
        "--hostname",
        "127.0.0.1",
        "--port",
        String(ports.web),
      ],
      env: webEnvironment,
    },
  ];
  const services = [
    {
      ...localURL("facilitator", "http", "127.0.0.1", ports.facilitator, "/verify"),
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ paymentSignature: "mock:approved" }),
      expectedStatuses: [200],
    },
    {
      ...localURL("seller", "http", "127.0.0.1", ports.seller, "/research/basic"),
      method: "POST",
      headers: { "content-type": "application/json" },
      body: "{}",
      expectedStatuses: [200],
    },
    {
      ...localURL("api", "http", "127.0.0.1", ports.api, "/health/ready"),
      method: "GET",
      expectedStatuses: [200],
    },
    {
      ...localURL("web", "http", "localhost", ports.web, "/"),
      method: "GET",
      expectedStatuses: [200, 307, 308],
    },
  ];
  const config = {
    repositoryRoot,
    goBuildCache,
    platform,
    ports,
    webOrigin,
    apiOrigin,
    repositoryMode: "memory",
    buyerMode: "deterministic",
    processes,
    services,
    prerequisites: [
      { command: "go", args: ["version"], label: "go" },
      { command: npmCommand, args: [...npmArgumentsPrefix, "--version"], label: "npm" },
    ],
    urls: {
      web: webOrigin,
      buyer: `${webOrigin}/buyer`,
      apiHealth: `${apiOrigin}/health/ready`,
      seller: `http://127.0.0.1:${ports.seller}/research/basic`,
      facilitator: `http://127.0.0.1:${ports.facilitator}/verify`,
      approvalWebSocket: `ws://127.0.0.1:${ports.api}/ws/approval-sessions/{sessionId}`,
      seedProfile: `${apiOrigin}/__dev/seed-profile`,
      seedReset: `${apiOrigin}/__dev/seed-profile/reset`,
    },
  };
  validateLocalRuntimeConfig(config);
  return config;
}

function assertLoopbackOrigin(rawOrigin, label) {
  let origin;
  try {
    origin = new URL(rawOrigin);
  } catch {
    throw new Error(`${label} origin must be a valid URL.`);
  }
  if (origin.protocol !== "http:" || !["localhost", "127.0.0.1", "::1"].includes(origin.hostname)) {
    throw new Error(`${label} origin must use localhost or a loopback address.`);
  }
}

// validateLocalRuntimeConfig prevents accidental non-local or ambiguous launches.
export function validateLocalRuntimeConfig(config) {
  assertLoopbackOrigin(config.webOrigin, "web");
  assertLoopbackOrigin(config.apiOrigin, "api");
  if (config.repositoryMode !== "memory") {
    throw new Error("local runtime repository mode must be memory.");
  }
  if (config.buyerMode !== "deterministic") {
    throw new Error("local runtime buyer mode must be deterministic.");
  }

  const assignedPorts = new Map();
  for (const [name, port] of Object.entries(config.ports)) {
    if (!Number.isInteger(port) || port < 1 || port > 65_535) {
      throw new Error(`${name} port must be an integer from 1 through 65535.`);
    }
    const priorName = assignedPorts.get(port);
    if (priorName) {
      throw new Error(`Port ${port} is assigned to both ${priorName} and ${name}.`);
    }
    assignedPorts.set(port, name);
  }
}

function delay(milliseconds) {
  return new Promise((resolveDelay) => setTimeout(resolveDelay, milliseconds));
}

async function defaultProbe(service) {
  try {
    const response = await fetch(service.url, {
      method: service.method,
      headers: service.headers,
      body: service.body,
      redirect: "manual",
      signal: AbortSignal.timeout(2_000),
    });
    return service.expectedStatuses.includes(response.status);
  } catch {
    return false;
  }
}

async function waitForService(service, options) {
  const probe = options.probe ?? defaultProbe;
  const retryDelayMilliseconds = options.retryDelayMilliseconds ?? defaultRetryDelayMilliseconds;
  const readinessTimeoutMilliseconds =
    options.readinessTimeoutMilliseconds ?? defaultReadinessTimeoutMilliseconds;
  const deadline = Date.now() + readinessTimeoutMilliseconds;

  while (Date.now() <= deadline) {
    if (await probe(service)) {
      return;
    }
    await delay(retryDelayMilliseconds);
  }
  throw new Error(`${service.name} failed readiness at ${service.url}.`);
}

// waitForServices waits for all transaction-path processes, not only the web port.
export async function waitForServices(services, options = {}) {
  await Promise.all(services.map((service) => waitForService(service, options)));
}

function checkPortAvailable(host, port) {
  return new Promise((resolveCheck, rejectCheck) => {
    const server = createServer();
    server.unref();
    server.once("error", (error) => rejectCheck(new Error(`Port ${port} is unavailable: ${error.message}`)));
    server.listen({ host, port, exclusive: true }, () => {
      server.close((error) => (error ? rejectCheck(error) : resolveCheck()));
    });
  });
}

async function validatePrerequisites(config) {
  mkdirSync(config.goBuildCache, { recursive: true });
  for (const { command, args, label } of config.prerequisites) {
    const result = spawnSync(command, args, {
      cwd: config.repositoryRoot,
      env: process.env,
      stdio: "ignore",
    });
    if (result.error || result.status !== 0) {
      throw new Error(`Required command ${label} is unavailable.`);
    }
  }
  await Promise.all(Object.values(config.ports).map((port) => checkPortAvailable("127.0.0.1", port)));
}

function forwardOutput(stream, name, destination) {
  let pending = "";
  stream?.setEncoding("utf8");
  stream?.on("data", (chunk) => {
    pending += chunk;
    const lines = pending.split(/\r?\n/);
    pending = lines.pop() ?? "";
    for (const line of lines) {
      if (line) destination.write(`[${name}] ${line}\n`);
    }
  });
  stream?.on("end", () => {
    if (pending) destination.write(`[${name}] ${pending}\n`);
  });
}

function spawnProcesses(config, output, errorOutput) {
  return config.processes.map((specification) => {
    const childProcess = spawn(specification.command, specification.args, {
      cwd: config.repositoryRoot,
      detached: config.platform !== "win32",
      env: { ...process.env, ...specification.env },
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true,
    });
    forwardOutput(childProcess.stdout, specification.name, output);
    forwardOutput(childProcess.stderr, specification.name, errorOutput);
    return { name: specification.name, process: childProcess };
  });
}

function processFailure(children, isShuttingDown) {
  return new Promise((_, rejectFailure) => {
    for (const child of children) {
      child.process.once("error", (error) => {
        if (isShuttingDown()) return;
        rejectFailure(new Error(`${child.name} failed to start: ${error.message}`));
      });
      child.process.once("exit", (code, signal) => {
        if (isShuttingDown()) return;
        rejectFailure(
          new Error(`${child.name} exited unexpectedly (${signal ? `signal ${signal}` : `code ${code}`}).`),
        );
      });
    }
  });
}

// collectDescendantProcessIDs returns a leaf-first snapshot to avoid orphaning children.
export function collectDescendantProcessIDs(processes, rootProcessID) {
  const childrenByParent = new Map();
  for (const processRecord of processes) {
    const children = childrenByParent.get(processRecord.parentProcessId) ?? [];
    children.push(processRecord.processId);
    childrenByParent.set(processRecord.parentProcessId, children);
  }

  const orderedProcessIDs = [];
  const visit = (processID) => {
    for (const childProcessID of childrenByParent.get(processID) ?? []) {
      visit(childProcessID);
    }
    orderedProcessIDs.push(processID);
  };
  visit(rootProcessID);
  return orderedProcessIDs;
}

// expandKnownProcessIDs accumulates descendants across snapshots as wrappers exit.
export function expandKnownProcessIDs(knownProcessIDs, processes) {
  const known = new Set(knownProcessIDs);
  let expanded = true;
  while (expanded) {
    expanded = false;
    for (const processRecord of processes) {
      if (known.has(processRecord.parentProcessId) && !known.has(processRecord.processId)) {
        known.add(processRecord.processId);
        expanded = true;
      }
    }
  }
  return [...known];
}

function readWindowsProcesses() {
  const result = spawnSync(
    "powershell.exe",
    [
      "-NoProfile",
      "-NonInteractive",
      "-Command",
      "Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId | ConvertTo-Json -Compress",
    ],
    { encoding: "utf8", windowsHide: true },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`Windows process-tree inspection failed with status ${result.status}.`);
  }
  const records = JSON.parse(result.stdout || "[]");
  return (Array.isArray(records) ? records : [records]).map((record) => ({
    processId: Number(record.ProcessId),
    parentProcessId: Number(record.ParentProcessId),
  }));
}

function terminateWindowsProcess(processID) {
  const result = spawnSync("taskkill.exe", ["/PID", String(processID), "/F"], {
    encoding: "utf8",
    windowsHide: true,
  });
  if (result.error) throw result.error;
  if (result.status === 0) return;

  const detail = `${result.stdout ?? ""} ${result.stderr ?? ""}`.trim();
  if (/not found|no running instance/i.test(detail)) return;
  throw new Error(
    `taskkill failed for PID ${processID} with status ${result.status}${detail ? `: ${detail}` : "."}`,
  );
}

// createCleanupGuardianArguments passes no credentials to the detached watchdog.
export function createCleanupGuardianArguments(launcherProcessID, children) {
  if (!Number.isInteger(launcherProcessID) || launcherProcessID <= 0) {
    throw new Error("Launcher process ID must be a positive integer.");
  }
  const childProcessIDs = children.map((child) => child.process.pid);
  if (childProcessIDs.some((processID) => !Number.isInteger(processID) || processID <= 0)) {
    throw new Error("Every local service must have a positive process ID.");
  }
  return [
    "--cleanup-guardian",
    String(launcherProcessID),
    ...childProcessIDs.map(String),
  ];
}

function processIsRunning(processID) {
  try {
    process.kill(processID, 0);
    return true;
  } catch (error) {
    return error?.code === "EPERM";
  }
}

async function runCleanupGuardian(argumentsList) {
  const [launcherValue, ...serviceValues] = argumentsList;
  const launcherProcessID = Number(launcherValue);
  const serviceProcessIDs = serviceValues.map(Number);
  if (
    !Number.isInteger(launcherProcessID) ||
    launcherProcessID <= 0 ||
    serviceProcessIDs.some((processID) => !Number.isInteger(processID) || processID <= 0)
  ) {
    return 1;
  }

  let knownProcessIDs = [...serviceProcessIDs];
  for (const snapshotDelay of guardianSnapshotDelaysMilliseconds) {
    if (snapshotDelay > 0) await delay(snapshotDelay);
    if (!processIsRunning(launcherProcessID)) break;
    try {
      knownProcessIDs = expandKnownProcessIDs(knownProcessIDs, readWindowsProcesses());
    } catch {
      // The final snapshot still gets a chance after the launcher exits.
    }
  }
  while (processIsRunning(launcherProcessID)) {
    await delay(guardianParentPollMilliseconds);
  }
  try {
    knownProcessIDs = expandKnownProcessIDs(knownProcessIDs, readWindowsProcesses());
  } catch {
    // Known descendants are still terminated when the last snapshot is unavailable.
  }
  let failed = false;
  for (const serviceProcessID of knownProcessIDs.reverse()) {
    try {
      terminateWindowsProcess(serviceProcessID);
    } catch {
      failed = true;
    }
  }
  return failed ? 1 : 0;
}

function startCleanupGuardian(config, children) {
  if (config.platform !== "win32") return;
  const guardianArguments = createCleanupGuardianArguments(process.pid, children);
  const guardian = spawn(
    process.execPath,
    [fileURLToPath(import.meta.url), ...guardianArguments],
    {
      cwd: config.repositoryRoot,
      detached: true,
      env: process.env,
      stdio: "ignore",
      windowsHide: true,
    },
  );
  guardian.unref();
}

function defaultTerminateTree(pid, platform) {
  if (platform === "win32") {
    const processIDs = collectDescendantProcessIDs(readWindowsProcesses(), pid);
    for (const processID of processIDs) {
      terminateWindowsProcess(processID);
    }
    return;
  }
  process.kill(-pid, "SIGTERM");
}

// shutdownChildren terminates descendant processes so Ctrl+C cannot orphan Go or Next.js.
export async function shutdownChildren(children, options = {}) {
  const platform = options.platform ?? process.platform;
  const terminateTree = options.terminateTree ?? ((pid) => defaultTerminateTree(pid, platform));
  await Promise.all(
    children.map(async (child) => {
      if (!child.process.pid) return;
      try {
        await terminateTree(child.process.pid);
      } catch (error) {
        // A child may exit between the exit-code check and process-tree termination.
        if (error?.code !== "ESRCH" && !/not found|no running instance/i.test(error?.message ?? "")) {
          throw error;
        }
      }
    }),
  );
}

// installShutdownHandlers makes the first termination signal authoritative.
export function installShutdownHandlers({ signalSource = process, shutdown }) {
  let shutdownRequested = false;
  const listeners = new Map();
  for (const signal of ["SIGINT", "SIGTERM"]) {
    const listener = () => {
      if (shutdownRequested) return;
      shutdownRequested = true;
      void Promise.resolve(shutdown(signal));
    };
    listeners.set(signal, listener);
    signalSource.on(signal, listener);
  }
  return () => {
    for (const [signal, listener] of listeners) signalSource.off(signal, listener);
  };
}

function printReady(config, output) {
  output.write("\nAgentPay local runtime is ready. Data is in memory and disappears on shutdown.\n");
  output.write(`Web:                 ${config.urls.web}\n`);
  output.write(`Deterministic buyer: ${config.urls.buyer}\n`);
  output.write(`API health:          ${config.urls.apiHealth}\n`);
  output.write(`Demo seller:         ${config.urls.seller}\n`);
  output.write(`Mock facilitator:    ${config.urls.facilitator}\n`);
  output.write(`Approval WebSocket:  ${config.urls.approvalWebSocket}\n`);
  output.write(`Seed profile:        ${config.urls.seedProfile}\n`);
  output.write(`Reset seed (POST):   ${config.urls.seedReset}\n`);
  output.write("Press Ctrl+C once to stop every process.\n\n");
}

// runLocalRuntime supervises the complete disposable local process graph.
export async function runLocalRuntime(config = createLocalRuntimeConfig(), options = {}) {
  const output = options.output ?? process.stdout;
  const errorOutput = options.errorOutput ?? process.stderr;
  const signalSource = options.signalSource ?? process;
  const prerequisiteValidator = options.validatePrerequisites ?? validatePrerequisites;
  const processSpawner = options.spawnProcesses ?? spawnProcesses;
  const cleanupGuardianStarter = options.startCleanupGuardian ?? startCleanupGuardian;
  const readinessWaiter = options.waitForServices ?? waitForServices;
  const childShutdown = options.shutdownChildren ?? shutdownChildren;
  const children = [];
  let resolveStop;
  const stopRequested = new Promise((resolveStopRequest) => {
    resolveStop = resolveStopRequest;
  });
  let shutdownPromise;
  let shutdownStarted = false;
  const shutdown = (reason) => {
    if (!shutdownPromise) {
      shutdownStarted = true;
      output.write(`\nStopping AgentPay local runtime (${reason})...\n`);
      shutdownPromise = childShutdown(children, { platform: config.platform }).then(() => {
        resolveStop({ type: "stopped" });
      });
    }
    return shutdownPromise;
  };
  const removeSignalHandlers = installShutdownHandlers({ signalSource, shutdown });

  try {
    await prerequisiteValidator(config);
    if (shutdownStarted) {
      await shutdownPromise;
      return 0;
    }
    children.push(...processSpawner(config, output, errorOutput));
    cleanupGuardianStarter(config, children);
    const failure = processFailure(children, () => shutdownStarted);
    const readiness = readinessWaiter(config.services).then(() => ({ type: "ready" }));
    const startupResult = await Promise.race([readiness, failure, stopRequested]);
    if (startupResult.type === "stopped") return 0;

    printReady(config, output);
    await Promise.race([failure, stopRequested]);
    return 0;
  } catch (error) {
    errorOutput.write(`AgentPay local runtime failed: ${error.message}\n`);
    await shutdown("failure");
    return 1;
  } finally {
    removeSignalHandlers();
  }
}

const invokedAsScript = process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href;
if (invokedAsScript && process.argv[2] === "--cleanup-guardian") {
  process.exitCode = await runCleanupGuardian(process.argv.slice(3));
} else if (invokedAsScript) {
  process.exitCode = await runLocalRuntime();
}
