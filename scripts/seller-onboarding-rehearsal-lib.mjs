import { createHash, createPublicKey, randomBytes, verify } from "node:crypto";

const acknowledgement = "create-clean-test-account";
const secretFieldPattern = /(authorization|cookie|password|private|projectkey|proof|secret|signature|token)/iu;
const secretValuePattern = /(Bearer\s+|apc[12]\.|mcg1\.|eyJ[A-Za-z0-9_-]+\.)/u;
const emailPattern = /[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/giu;
const longHexPattern = /0x[a-f0-9]{64,}/giu;

function required(environment, name) {
  const value = environment[name]?.trim();
  if (!value) throw new Error(`${name} is required.`);
  return value;
}

function httpsOrigin(environment, name) {
  const raw = required(environment, name).replace(/\/$/u, "");
  const url = new URL(raw);
  if (url.protocol !== "https:" || url.pathname !== "/") {
    throw new Error(`${name} must be an HTTPS origin without a path.`);
  }
  return url.origin;
}

function positiveAtomicAmount(environment, name) {
  const value = required(environment, name);
  if (!/^[1-9][0-9]*$/u.test(value)) {
    throw new Error(`${name} must be a positive atomic-unit integer string.`);
  }
  return value;
}

export function requireRehearsalConfiguration(environment = process.env) {
  if (environment.AGENTPAY_REHEARSAL_ACK !== acknowledgement) {
    throw new Error(
      `AGENTPAY_REHEARSAL_ACK must equal ${acknowledgement}.`,
    );
  }
  const email = required(environment, "AGENTPAY_REHEARSAL_EMAIL").toLowerCase();
  if (!email.includes("@")) {
    throw new Error("AGENTPAY_REHEARSAL_EMAIL must be a valid test recipient.");
  }
  const payoutPrivateKey = required(
    environment,
    "AGENTPAY_REHEARSAL_PAYOUT_PRIVATE_KEY",
  );
  if (!/^0x[a-fA-F0-9]{64}$/u.test(payoutPrivateKey)) {
    throw new Error(
      "AGENTPAY_REHEARSAL_PAYOUT_PRIVATE_KEY must be a 32-byte hex key.",
    );
  }
  const routePath = required(environment, "AGENTPAY_REHEARSAL_ROUTE_PATH");
  if (!/^\/[A-Za-z0-9/_-]+$/u.test(routePath)) {
    throw new Error("AGENTPAY_REHEARSAL_ROUTE_PATH must be a literal API path.");
  }
  const routeMethod = (
    environment.AGENTPAY_REHEARSAL_ROUTE_METHOD ?? "POST"
  ).toUpperCase();
  if (!new Set(["GET", "POST"]).has(routeMethod)) {
    throw new Error("AGENTPAY_REHEARSAL_ROUTE_METHOD must be GET or POST.");
  }
  const apiOrigin = httpsOrigin(
    environment,
    "AGENTPAY_REHEARSAL_API_ORIGIN",
  );
  const webOrigin = httpsOrigin(
    environment,
    "AGENTPAY_REHEARSAL_WEB_ORIGIN",
  );
  const serviceOrigin = httpsOrigin(
    environment,
    "AGENTPAY_REHEARSAL_SERVICE_ORIGIN",
  );
  const configuration = {
    apiOrigin,
    webOrigin,
    serviceOrigin,
    email,
    region: required(environment, "AWS_REGION"),
    cognitoClientId: required(
      environment,
      "AGENTPAY_REHEARSAL_COGNITO_CLIENT_ID",
    ),
    payoutPrivateKey,
    manifestPath: required(
      environment,
      "AGENTPAY_REHEARSAL_MANIFEST_PATH",
    ),
    openapiPath: required(environment, "AGENTPAY_REHEARSAL_OPENAPI_PATH"),
    framework: required(environment, "AGENTPAY_REHEARSAL_FRAMEWORK"),
    expectedStack: required(environment, "AGENTPAY_REHEARSAL_EXPECTED_STACK"),
    route: {
      amount: positiveAtomicAmount(
        environment,
        "AGENTPAY_REHEARSAL_ROUTE_AMOUNT",
      ),
      displayName: required(
        environment,
        "AGENTPAY_REHEARSAL_ROUTE_DISPLAY_NAME",
      ),
      method: routeMethod,
      path: routePath,
      slug: required(environment, "AGENTPAY_REHEARSAL_ROUTE_SLUG"),
    },
    entitlementWaitMilliseconds: Number(
      environment.AGENTPAY_REHEARSAL_ENTITLEMENT_WAIT_MS ?? 1_800_000,
    ),
    evidencePath:
      environment.AGENTPAY_REHEARSAL_EVIDENCE_PATH?.trim() ||
      `.cache/rehearsals/seller-onboarding-${Date.now()}.json`,
  };
  if (
    !Number.isSafeInteger(configuration.entitlementWaitMilliseconds) ||
    configuration.entitlementWaitMilliseconds < 1_000
  ) {
    throw new Error(
      "AGENTPAY_REHEARSAL_ENTITLEMENT_WAIT_MS must be an integer of at least 1000.",
    );
  }
  return {
    ...configuration,
    publicSummary: {
      apiOrigin,
      webOrigin,
      serviceOrigin,
      email,
      routeMethod,
      routePath,
      evidencePath: configuration.evidencePath,
    },
  };
}

function canonicalize(value) {
  if (value === null || typeof value === "boolean" || typeof value === "string") {
    return JSON.stringify(value);
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new Error("Cannot canonicalize non-finite numbers.");
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map(canonicalize).join(",")}]`;
  }
  if (typeof value === "object") {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonicalize(value[key])}`)
      .join(",")}}`;
  }
  throw new Error("Cannot canonicalize this value.");
}

export function canonicalMutationHash(argumentsValue) {
  const clean = { ...argumentsValue };
  delete clean.idempotencyKey;
  delete clean.confirmationGrant;
  return createHash("sha256")
    .update("agentpay.mcp-mutation.v1\0")
    .update(canonicalize(clean))
    .digest("hex");
}

export function verifySignedDiscovery(envelope, jwks) {
  const signature = envelope?.signature;
  if (
    !envelope?.document ||
    signature?.alg !== "ES256" ||
    signature?.canonicalization !== "RFC8785" ||
    signature?.domainSeparator !== "agentpay.discovery.v1" ||
    typeof signature.kid !== "string" ||
    typeof signature.value !== "string" ||
    !Array.isArray(jwks?.keys)
  ) {
    return false;
  }
  const jwk = jwks.keys.find(
    (candidate) =>
      candidate?.kid === signature.kid &&
      candidate?.kty === "EC" &&
      candidate?.crv === "P-256",
  );
  if (!jwk) return false;
  try {
    const payload = Buffer.from(
      `${signature.domainSeparator}\0${canonicalize(envelope.document)}`,
      "utf8",
    );
    return verify(
      "sha256",
      payload,
      { key: createPublicKey({ key: jwk, format: "jwk" }), dsaEncoding: "ieee-p1363" },
      Buffer.from(signature.value, "base64url"),
    );
  } catch {
    return false;
  }
}

export function generateRehearsalPassword() {
  return `Ap-${randomBytes(24).toString("base64url")}9!`;
}

export function assertExactConfirmation(actual, expected) {
  if (actual !== expected) {
    throw new Error("Seller confirmation did not match the reviewed mutation.");
  }
}

export async function runStep(name, operation, onPass) {
  try {
    const result = await operation();
    onPass?.(result);
    return result;
  } catch (error) {
    const message = safeDiagnosticMessage(
      error instanceof Error ? error.message : String(error),
    );
    throw new Error(`${name} failed: ${message}`, { cause: error });
  }
}

export function safeDiagnosticMessage(message) {
  return String(message)
    .replace(emailPattern, "[redacted-email]")
    .replace(longHexPattern, "[redacted-hex]")
    .replace(/Bearer\s+\S+/giu, "Bearer [redacted]")
    .replace(/(?:apc[12]|mcg1)\.[A-Za-z0-9_.-]+/gu, "[redacted-credential]")
    .replace(/eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/gu, "[redacted-jwt]");
}

function assertSanitized(details) {
  for (const [key, value] of Object.entries(details)) {
    if (secretFieldPattern.test(key)) {
      throw new Error(`Refusing secret-bearing evidence field: ${key}.`);
    }
    if (typeof value === "string" && secretValuePattern.test(value)) {
      throw new Error(`Refusing secret-like evidence value in ${key}.`);
    }
    if (
      value !== null &&
      !["string", "number", "boolean"].includes(typeof value)
    ) {
      throw new Error(`Evidence field ${key} must be a scalar value.`);
    }
  }
}

export function createEvidenceRecorder(metadata) {
  assertSanitized(metadata);
  const evidence = {
    schemaVersion: "agentpay.clean-seller-rehearsal.v1",
    startedAt: new Date().toISOString(),
    ...metadata,
    steps: [],
  };
  return {
    pass(name, details = {}) {
      assertSanitized(details);
      evidence.steps.push({ name, status: "passed", ...details });
    },
    document() {
      return { ...evidence, steps: evidence.steps.map((step) => ({ ...step })) };
    },
  };
}
