import {
  CognitoIdentityProviderClient,
  InitiateAuthCommand,
} from "@aws-sdk/client-cognito-identity-provider";
import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";
import { Wallet } from "ethers";

import { CookieJar } from "./deployed-auth-smoke-lib.mjs";
import {
  assertExactConfirmation,
  canonicalMutationHash,
  createEvidenceRecorder,
  generateRehearsalPassword,
  requireRehearsalConfiguration,
  runStep,
  safeDiagnosticMessage,
  verifySignedDiscovery,
} from "./seller-onboarding-rehearsal-lib.mjs";

const requestTimeoutMilliseconds = 20_000;
const entitlementPollMilliseconds = 5_000;
const maximumResponseBytes = 1_048_576;
const asset = "USDC";
const network = "eip155:84532";

function pass(name) {
  stdout.write(`[PASS] ${name}\n`);
}

async function prompt(message) {
  const terminal = createInterface({ input: stdin, output: stdout });
  try {
    return (await terminal.question(message)).trim();
  } finally {
    terminal.close();
  }
}

async function readJson(response, step) {
  const contentLength = Number(response.headers.get("content-length"));
  if (Number.isFinite(contentLength) && contentLength > maximumResponseBytes) {
    throw new Error(`${step} response exceeded the allowed size.`);
  }
  const body = await response.text();
  if (Buffer.byteLength(body, "utf8") > maximumResponseBytes) {
    throw new Error(`${step} response exceeded the allowed size.`);
  }
  try {
    return body ? JSON.parse(body) : {};
  } catch {
    throw new Error(`${step} returned invalid JSON with status ${response.status}.`);
  }
}

async function apiRequest(configuration, accessToken, step, requestPath, options = {}) {
  const response = await fetch(`${configuration.apiOrigin}${requestPath}`, {
    method: options.method ?? "GET",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${accessToken}`,
      ...(options.body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(options.mutation ? { "Idempotency-Key": randomUUID() } : {}),
    },
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    cache: "no-store",
    redirect: "error",
    signal: AbortSignal.timeout(requestTimeoutMilliseconds),
  });
  const responseBody = await readJson(response, step);
  if (!response.ok) {
    const code = responseBody?.error?.code;
    throw new Error(
      `HTTP ${response.status}${typeof code === "string" ? ` (${code})` : ""}`,
    );
  }
  return responseBody;
}

async function publicRequest(configuration, step, requestPath) {
  const response = await fetch(`${configuration.apiOrigin}${requestPath}`, {
    headers: { Accept: "application/json" },
    cache: "no-store",
    redirect: "error",
    signal: AbortSignal.timeout(requestTimeoutMilliseconds),
  });
  const responseBody = await readJson(response, step);
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return responseBody;
}

class StdioMcpClient {
  constructor(configuration, projectKey) {
    const connectorPath = path.resolve(
      "packages/local-mcp-connector/dist/cli.js",
    );
    const inheritedEnvironment = Object.fromEntries(
      [
        "HOME",
        "HTTPS_PROXY",
        "HTTP_PROXY",
        "NODE_EXTRA_CA_CERTS",
        "NO_PROXY",
        "PATH",
        "PATHEXT",
        "SSL_CERT_FILE",
        "SYSTEMROOT",
        "TEMP",
        "TMP",
        "USERPROFILE",
        "WINDIR",
      ]
        .map((name) => [name, process.env[name]])
        .filter((entry) => typeof entry[1] === "string"),
    );
    this.child = spawn(process.execPath, [connectorPath], {
      env: {
        ...inheritedEnvironment,
        AGENTPAY_API_BASE_URL: configuration.apiOrigin,
        AGENTPAY_PROJECT_KEY: projectKey,
        AGENTPAY_MCP_SCOPES: "read configure validate publish",
      },
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
    });
    this.pending = new Map();
    this.buffer = "";
    this.diagnostic = "";
    this.child.stdout.setEncoding("utf8");
    this.child.stderr.setEncoding("utf8");
    this.child.stdout.on("data", (chunk) => this.#receive(chunk));
    this.child.stderr.on("data", (chunk) => {
      if (!this.diagnostic) this.diagnostic = String(chunk).trim().slice(0, 256);
    });
    this.child.on("exit", () => {
      for (const pending of this.pending.values()) {
        pending.reject(new Error(this.diagnostic || "connector exited unexpectedly"));
      }
      this.pending.clear();
    });
  }

  #receive(chunk) {
    this.buffer += chunk;
    while (this.buffer.includes("\n")) {
      const newline = this.buffer.indexOf("\n");
      const line = this.buffer.slice(0, newline).trim();
      this.buffer = this.buffer.slice(newline + 1);
      if (!line) continue;
      let message;
      try {
        message = JSON.parse(line);
      } catch {
        continue;
      }
      const pending = this.pending.get(message.id);
      if (!pending) continue;
      clearTimeout(pending.timeout);
      this.pending.delete(message.id);
      if (message.error) pending.reject(new Error("MCP request was rejected."));
      else pending.resolve(message.result);
    }
  }

  request(method, params = {}) {
    const id = randomUUID();
    const message = JSON.stringify({ jsonrpc: "2.0", id, method, params });
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("MCP request timed out."));
      }, requestTimeoutMilliseconds);
      this.pending.set(id, { resolve, reject, timeout });
      this.child.stdin.write(`${message}\n`);
    });
  }

  notify(method, params = {}) {
    this.child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method, params })}\n`);
  }

  async initialize() {
    await this.request("initialize", {
      protocolVersion: "2026-07-28",
      capabilities: {},
      clientInfo: { name: "agentpay-clean-seller-rehearsal", version: "1.0.0" },
    });
    this.notify("notifications/initialized");
  }

  async callTool(name, argumentsValue) {
    const result = await this.request("tools/call", {
      name,
      arguments: argumentsValue,
    });
    if (result?.isError) throw new Error(`${name} returned an MCP tool error.`);
    if (!result?.structuredContent || typeof result.structuredContent !== "object") {
      throw new Error(`${name} returned no structured result.`);
    }
    return result.structuredContent;
  }

  close() {
    this.child.stdin.end();
    this.child.kill();
  }
}

async function waitForEntitlement(configuration, accessToken, sellerId) {
  stdout.write(
    `Operator action required for seller ${sellerId}. Grant a bounded launch entitlement using docs/runbooks/LAUNCH_ENTITLEMENT.md.\n`,
  );
  const deadline = Date.now() + configuration.entitlementWaitMilliseconds;
  while (Date.now() < deadline) {
    try {
      const plan = await apiRequest(
        configuration,
        accessToken,
        "Launch entitlement",
        `/v1/sellers/${encodeURIComponent(sellerId)}/plan`,
      );
      if (
        plan.assignment?.status === "active" &&
        plan.assignment?.networkAccess === "enabled"
      ) {
        return plan;
      }
    } catch {
      // A clean seller has no entitlement until the separate operator acts.
    }
    await new Promise((resolve) => setTimeout(resolve, entitlementPollMilliseconds));
  }
  throw new Error("timed out waiting for the manual launch entitlement");
}

async function createConfirmationGrant(
  configuration,
  accessToken,
  sellerId,
  credentialId,
  tool,
  targetType,
  targetId,
  expectedResourceVersion,
  argumentsValue,
  summary,
) {
  return apiRequest(
    configuration,
    accessToken,
    `${tool} confirmation`,
    `/v1/sellers/${encodeURIComponent(sellerId)}/mcp-confirmation-grants`,
    {
      method: "POST",
      mutation: true,
      body: {
        credentialId,
        tool,
        targetType,
        targetId,
        argumentsSha256: canonicalMutationHash(argumentsValue),
        expectedResourceVersion,
        summary,
      },
    },
  );
}

async function confirmCommercialMutation(tool, route, details) {
  stdout.write(`\nReview ${tool}:\n${JSON.stringify(details, null, 2)}\n`);
  const expected = `CONFIRM ${tool} ${route.slug}`;
  const actual = await prompt(`Type exactly "${expected}" to continue: `);
  assertExactConfirmation(actual, expected);
}

async function verifyBrowserDashboard(configuration, email, password) {
  const cookies = new CookieJar();
  async function webRequest(requestPath, options = {}) {
    const response = await fetch(`${configuration.webOrigin}${requestPath}`, {
      ...options,
      redirect: options.redirect ?? "manual",
      headers: {
        ...(cookies.header() ? { Cookie: cookies.header() } : {}),
        ...options.headers,
      },
    });
    cookies.capture(response.headers);
    return response;
  }
  const csrfResponse = await webRequest("/api/auth/csrf");
  if (csrfResponse.status !== 200) throw new Error(`CSRF bootstrap returned ${csrfResponse.status}.`);
  const csrf = await readJson(csrfResponse, "Dashboard CSRF bootstrap");
  const signIn = await webRequest("/api/auth/sign-in", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Origin: configuration.webOrigin,
      "X-AgentPay-CSRF": csrf.csrfToken,
    },
    body: JSON.stringify({ email, password, returnTo: "/dashboard" }),
  });
  if (signIn.status !== 200) throw new Error(`Browser sign-in returned ${signIn.status}.`);
  const signInBody = await readJson(signIn, "Dashboard browser sign-in");
  if (signInBody.redirectTo !== "/dashboard") {
    throw new Error(
      `Browser sign-in returned ${signInBody.redirectTo ?? "no redirect"} instead of dashboard readiness.`,
    );
  }
  const dashboard = await webRequest("/dashboard", { redirect: "follow" });
  if (
    dashboard.status !== 200 ||
    new URL(dashboard.url).pathname.replace(/\/$/u, "") !== "/dashboard"
  ) {
    throw new Error(`Dashboard returned ${dashboard.status}.`);
  }
}

async function signUpAndVerifyThroughWeb(configuration, email, password) {
  const cookies = new CookieJar();
  async function webRequest(requestPath, options = {}) {
    const response = await fetch(`${configuration.webOrigin}${requestPath}`, {
      ...options,
      redirect: options.redirect ?? "manual",
      headers: {
        ...(cookies.header() ? { Cookie: cookies.header() } : {}),
        ...options.headers,
      },
    });
    cookies.capture(response.headers);
    return response;
  }
  async function post(requestPath, body) {
    const csrfResponse = await webRequest("/api/auth/csrf");
    if (csrfResponse.status !== 200) {
      throw new Error(`CSRF bootstrap returned ${csrfResponse.status}.`);
    }
    const csrf = await readJson(csrfResponse, "Authentication CSRF bootstrap");
    const response = await webRequest(requestPath, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Origin: configuration.webOrigin,
        "X-AgentPay-CSRF": csrf.csrfToken,
      },
      body: JSON.stringify(body),
    });
    if (response.status !== 200) {
      throw new Error(`${requestPath} returned ${response.status}.`);
    }
    return readJson(response, requestPath);
  }
  await post("/api/auth/sign-up", {
    email,
    name: "AgentPay clean-account rehearsal",
    password,
    returnTo: "/dashboard",
  });
  const verificationCode =
    process.env.AGENTPAY_REHEARSAL_VERIFICATION_CODE?.trim() ||
    (await prompt("Enter the six-digit Cognito code from the private test inbox: "));
  if (!/^\d{6}$/u.test(verificationCode)) {
    throw new Error("Verification code must contain six digits.");
  }
  await post("/api/auth/verify", {
    code: verificationCode,
    returnTo: "/dashboard",
  });
}

async function run() {
  const configuration = requireRehearsalConfiguration();
  const password = generateRehearsalPassword();
  const wallet = new Wallet(configuration.payoutPrivateKey);
  const commit = process.env.GITHUB_SHA?.trim() || "local-uncommitted-run";
  const evidence = createEvidenceRecorder({
    commit,
    apiOrigin: configuration.apiOrigin,
    webOrigin: configuration.webOrigin,
    serviceOrigin: configuration.serviceOrigin,
  });
  const cognito = new CognitoIdentityProviderClient({ region: configuration.region });
  let mcp;

  try {
    await runStep("Clean seller sign-up and email verification", () =>
      signUpAndVerifyThroughWeb(configuration, configuration.email, password),
    );
    pass("Clean seller sign-up and email verification");
    evidence.pass("Clean seller sign-up and email verification");

    const authentication = await runStep("Seller authentication", async () => {
      const response = await cognito.send(
        new InitiateAuthCommand({
          ClientId: configuration.cognitoClientId,
          AuthFlow: "USER_PASSWORD_AUTH",
          AuthParameters: { USERNAME: configuration.email, PASSWORD: password },
        }),
      );
      if (!response.AuthenticationResult?.AccessToken) {
        throw new Error("Cognito returned no access token.");
      }
      return response.AuthenticationResult;
    }, () => {
      pass("Seller authentication");
      evidence.pass("Seller authentication");
    });
    const accessToken = authentication.AccessToken;

    const slug = `rehearsal-${Date.now().toString(36)}-${randomBytes(3).toString("hex")}`;
    const seller = await runStep("Seller storefront creation", () =>
      apiRequest(configuration, accessToken, "Seller storefront creation", "/v1/sellers", {
        method: "POST",
        mutation: true,
        body: {
          name: "Clean Account Rehearsal",
          slug,
          upstreamBaseUrl: configuration.serviceOrigin,
        },
      }),
    );
    pass("Seller storefront creation");
    evidence.pass("Seller storefront creation", { sellerId: seller.sellerId, sellerSlug: slug });

    const activeSeller = await runStep("Seller service activation", () =>
      apiRequest(
        configuration,
        accessToken,
        "Seller service activation",
        `/v1/sellers/${encodeURIComponent(seller.sellerId)}/service-activation`,
        { method: "POST", mutation: true, body: { expectedVersion: seller.version } },
      ),
    );
    pass("Seller service activation");
    evidence.pass("Seller service activation", { sellerVersion: activeSeller.version });

    const destination = await runStep("Payout destination creation", () =>
      apiRequest(
        configuration,
        accessToken,
        "Payout destination creation",
        `/v1/sellers/${encodeURIComponent(seller.sellerId)}/payment-destinations`,
        { method: "POST", mutation: true, body: { asset, network, address: wallet.address } },
      ),
    );
    const ownership = await runStep("Payout ownership challenge", () =>
      apiRequest(
        configuration,
        accessToken,
        "Payout ownership challenge",
        `/v1/sellers/${encodeURIComponent(seller.sellerId)}/payment-destinations/${encodeURIComponent(destination.destinationId)}/ownership-challenges`,
        { method: "POST", mutation: true, body: {} },
      ),
    );
    const ownershipSignature = await wallet.signMessage(ownership.challenge);
    await runStep("Payout destination verification", () =>
      apiRequest(
        configuration,
        accessToken,
        "Payout destination verification",
        `/v1/sellers/${encodeURIComponent(seller.sellerId)}/payment-destinations/${encodeURIComponent(destination.destinationId)}/verify`,
        {
          method: "POST",
          mutation: true,
          body: { challenge: ownership.challenge, signature: ownershipSignature, confirmRotation: false },
        },
      ),
    );
    pass("Payout destination verification");
    evidence.pass("Payout destination verification", { destinationId: destination.destinationId, asset, network });

    await runStep("Manual launch entitlement", () =>
      waitForEntitlement(configuration, accessToken, seller.sellerId),
    );
    pass("Manual launch entitlement");
    evidence.pass("Manual launch entitlement");

    const credential = await runStep("Project-key creation", () =>
      apiRequest(
        configuration,
        accessToken,
        "Project-key creation",
        `/v1/sellers/${encodeURIComponent(seller.sellerId)}/integration-credentials`,
        {
          method: "POST",
          mutation: true,
          body: { label: "Clean-account rehearsal", scopes: ["read", "configure", "validate", "publish"], expiresAt: null },
        },
      ),
    );
    pass("Project-key creation");
    evidence.pass("Project-key creation", { credentialId: credential.credentialId });

    mcp = new StdioMcpClient(configuration, credential.token);
    await runStep("MCP connector authorization", () => mcp.initialize());
    pass("MCP connector authorization");
    evidence.pass("MCP connector authorization");

    const manifestContent = await readFile(configuration.manifestPath, "utf8");
    const openapiContent = await readFile(configuration.openapiPath, "utf8");
    const detection = await runStep("MCP route stack detection", () =>
      mcp.callTool("detect_repository_stacks", {
        files: { [path.basename(configuration.manifestPath)]: manifestContent },
      }),
    );
    if (!detection.detections?.some((entry) => entry.stack === configuration.expectedStack && entry.tier === "maintained")) {
      throw new Error(`MCP route stack detection failed: expected maintained stack ${configuration.expectedStack}.`);
    }
    pass("MCP route stack detection");
    evidence.pass("MCP route stack detection", { stack: configuration.expectedStack });

    const analysis = await runStep("MCP route analysis", () =>
      mcp.callTool("analyze_repository", {
        manifest: {
          schemaVersion: "agentpay.repository.v1",
          serviceName: "Clean Account Rehearsal",
          framework: configuration.framework,
          openapiPath: path.basename(configuration.openapiPath),
        },
        openapi: openapiContent,
      }),
    );
    const proposal = analysis.proposals?.find(
      (candidate) => candidate.path === configuration.route.path && candidate.method === configuration.route.method,
    );
    if (!proposal) throw new Error("MCP route analysis did not return the configured route.");
    pass("MCP route analysis");
    evidence.pass("MCP route analysis", { routeMethod: proposal.method, routePath: proposal.path });

    const routeConfiguration = {
      displayName: configuration.route.displayName,
      productSlug: configuration.route.slug,
      method: proposal.method,
      pathPattern: proposal.path,
      description: proposal.description || configuration.route.displayName,
      mimeType: proposal.mimeType || "application/json",
      inputSchema: { type: "object", properties: {}, additionalProperties: false },
      outputSchema: { type: "object", properties: {}, additionalProperties: false },
      amount: configuration.route.amount,
      asset,
      network,
      payTo: wallet.address,
      approvalThresholdAmount: null,
      upstreamTimeoutSeconds: 20,
    };
    const configureArguments = {
      idempotencyKey: randomUUID(),
      expectedSellerVersion: activeSeller.version,
      route: routeConfiguration,
    };
    await runStep("Seller confirmation for MCP route configuration", () =>
      confirmCommercialMutation("configure_route", configuration.route, {
        amount: configuration.route.amount,
        asset,
        method: configuration.route.method,
        network,
        path: configuration.route.path,
        slug: configuration.route.slug,
      }),
    );
    const configureGrant = await createConfirmationGrant(
      configuration,
      accessToken,
      seller.sellerId,
      credential.credentialId,
      "configure_route",
      "seller",
      seller.sellerId,
      activeSeller.version,
      configureArguments,
      `Create reviewed ${configuration.route.displayName} draft`,
    );
    const configured = await runStep("MCP route configuration", () =>
      mcp.callTool("configure_route", {
        ...configureArguments,
        confirmationGrant: configureGrant.confirmationGrant,
      }),
    );
    const route = configured.route;
    if (!route?.routeId || route.lifecycleStatus !== "draft") {
      throw new Error("MCP route configuration did not create a draft.");
    }
    pass("MCP route configuration");
    evidence.pass("MCP route configuration", { routeId: route.routeId, routeVersion: route.version });

    const validationResult = await runStep("MCP route validation", () =>
      mcp.callTool("validate_route", { routeId: route.routeId }),
    );
    const validation = validationResult.validation;
    if (!validation?.valid || !validation.contractHash) {
      throw new Error("MCP route validation did not pass all deterministic checks.");
    }
    pass("MCP route validation");
    evidence.pass("MCP route validation", { contractHash: validation.contractHash, routeVersion: validation.version });

    const sandbox = await runStep("MCP sandbox validation", () =>
      mcp.callTool("sandbox_validate_route", { routeId: route.routeId }),
    );
    if (!sandbox.valid) throw new Error("MCP sandbox validation did not pass.");
    pass("MCP sandbox validation");
    evidence.pass("MCP sandbox validation");

    const publishArguments = {
      idempotencyKey: randomUUID(),
      routeId: route.routeId,
      expectedVersion: validation.version,
      contractHash: validation.contractHash,
    };
    await runStep("Seller confirmation for MCP route publication", () =>
      confirmCommercialMutation("publish_route", configuration.route, {
        contractHash: validation.contractHash,
        routeId: route.routeId,
        routeVersion: validation.version,
        slug: configuration.route.slug,
      }),
    );
    const publishGrant = await createConfirmationGrant(
      configuration,
      accessToken,
      seller.sellerId,
      credential.credentialId,
      "publish_route",
      "paid_route",
      route.routeId,
      validation.version,
      publishArguments,
      `Publish reviewed ${configuration.route.displayName} product`,
    );
    const published = await runStep("MCP route publication", () =>
      mcp.callTool("publish_route", {
        ...publishArguments,
        confirmationGrant: publishGrant.confirmationGrant,
      }),
    );
    if (published.route?.lifecycleStatus !== "published" || !published.route?.enabled) {
      throw new Error("MCP publication did not return an enabled published route.");
    }
    pass("MCP route publication");
    evidence.pass("MCP route publication", { routeId: route.routeId, routeVersion: published.route.version });

    const manifest = await runStep("Signed storefront discovery", () =>
      publicRequest(configuration, "Signed storefront discovery", `/store/${encodeURIComponent(slug)}/manifest.json`),
    );
    const jwks = await publicRequest(
      configuration,
      "Discovery verification keys",
      "/.well-known/jwks.json",
    );
    const discovered = manifest.document?.products?.some((product) => product.routeId === route.routeId);
    if (!discovered || !verifySignedDiscovery(manifest, jwks)) {
      throw new Error("Signed discovery omitted the route or failed ES256 verification.");
    }
    pass("Signed storefront discovery");
    evidence.pass("Signed storefront discovery", { publicationRevision: manifest.document.publicationRevision, routeId: route.routeId });

    const dashboard = await runStep("Seller dashboard API readiness", () =>
      apiRequest(configuration, accessToken, "Seller dashboard API readiness", "/v1/me/dashboard"),
    );
    if (
      dashboard.seller?.sellerId !== seller.sellerId ||
      dashboard.products?.published < 1 ||
      dashboard.credentials?.active < 1 ||
      dashboard.onboarding?.publication?.allowed !== true
    ) {
      throw new Error("Seller dashboard summary is not launch-ready.");
    }
    pass("Seller dashboard API readiness");
    evidence.pass("Seller dashboard API readiness", { publishedProducts: dashboard.products.published, activeCredentials: dashboard.credentials.active });

    await runStep("Seller dashboard browser readiness", () =>
      verifyBrowserDashboard(configuration, configuration.email, password),
    );
    pass("Seller dashboard browser readiness");
    evidence.pass("Seller dashboard browser readiness");

    const evidenceDocument = { ...evidence.document(), completedAt: new Date().toISOString() };
    await mkdir(path.dirname(configuration.evidencePath), { recursive: true });
    await writeFile(configuration.evidencePath, `${JSON.stringify(evidenceDocument, null, 2)}\n`, {
      encoding: "utf8",
      mode: 0o600,
      flag: "wx",
    });
    stdout.write(`Clean-account seller rehearsal passed. Sanitized evidence: ${configuration.evidencePath}\n`);
  } finally {
    mcp?.close();
    cognito.destroy();
  }
}

run().catch((error) => {
  process.exitCode = 1;
  stdout.write(
    `Clean-account seller rehearsal failed: ${safeDiagnosticMessage(error instanceof Error ? error.message : String(error))}\n`,
  );
});
