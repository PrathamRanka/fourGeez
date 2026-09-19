import assert from "node:assert/strict";
import { generateKeyPairSync, sign } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  assertExactConfirmation,
  canonicalMutationHash,
  createEvidenceRecorder,
  requireRehearsalConfiguration,
  runStep,
  safeDiagnosticMessage,
  verifySignedDiscovery,
} from "./seller-onboarding-rehearsal-lib.mjs";

const validEnvironment = {
  AGENTPAY_REHEARSAL_ACK: "create-clean-test-account",
  AGENTPAY_REHEARSAL_API_ORIGIN: "https://api.agentpay.example",
  AGENTPAY_REHEARSAL_COGNITO_CLIENT_ID: "client-id",
  AGENTPAY_REHEARSAL_EMAIL: "clean-seller@example.test",
  AGENTPAY_REHEARSAL_EXPECTED_STACK: "express",
  AGENTPAY_REHEARSAL_FRAMEWORK: "node",
  AGENTPAY_REHEARSAL_MANIFEST_PATH: "fixtures/package.json",
  AGENTPAY_REHEARSAL_OPENAPI_PATH: "fixtures/openapi.yaml",
  AGENTPAY_REHEARSAL_PAYOUT_PRIVATE_KEY: `0x${"1".repeat(64)}`,
  AGENTPAY_REHEARSAL_ROUTE_AMOUNT: "100000",
  AGENTPAY_REHEARSAL_ROUTE_DISPLAY_NAME: "Research report",
  AGENTPAY_REHEARSAL_ROUTE_PATH: "/research",
  AGENTPAY_REHEARSAL_ROUTE_SLUG: "research-report",
  AGENTPAY_REHEARSAL_SERVICE_ORIGIN: "https://seller.example",
  AGENTPAY_REHEARSAL_WEB_ORIGIN: "https://agentpay.example",
  AWS_REGION: "ap-south-1",
};

test("rehearsal requires an explicit mutation acknowledgement and HTTPS origins", () => {
  assert.throws(
    () =>
      requireRehearsalConfiguration({
        ...validEnvironment,
        AGENTPAY_REHEARSAL_ACK: "",
      }),
    /AGENTPAY_REHEARSAL_ACK/,
  );
  assert.throws(
    () =>
      requireRehearsalConfiguration({
        ...validEnvironment,
        AGENTPAY_REHEARSAL_SERVICE_ORIGIN: "http://127.0.0.1:8090",
      }),
    /HTTPS/,
  );
});

test("rehearsal configuration keeps secrets out of the returned public summary", () => {
  const configuration = requireRehearsalConfiguration(validEnvironment);
  assert.equal(configuration.email, validEnvironment.AGENTPAY_REHEARSAL_EMAIL);
  assert.equal(
    JSON.stringify(configuration.publicSummary).includes(
      validEnvironment.AGENTPAY_REHEARSAL_PAYOUT_PRIVATE_KEY,
    ),
    false,
  );
});

test("mutation hashes are stable across object key order and omit transport secrets", () => {
  const first = canonicalMutationHash({
    idempotencyKey: "request-one",
    confirmationGrant: "mcg1.secret",
    expectedSellerVersion: 3,
    route: { method: "POST", amount: "100000" },
  });
  const second = canonicalMutationHash({
    route: { amount: "100000", method: "POST" },
    expectedSellerVersion: 3,
    confirmationGrant: "different-secret",
    idempotencyKey: "request-two",
  });
  assert.match(first, /^[a-f0-9]{64}$/);
  assert.equal(first, second);
  assert.equal(
    first,
    "e6aaedf44cb8ea80497e83a14092865724cc36a1aedd4a362ff97bce811fc796",
  );
});

test("step failures identify the exact named rehearsal step", async () => {
  await assert.rejects(
    runStep("MCP route validation", async () => {
      throw new Error("route was invalid");
    }),
    /MCP route validation failed: route was invalid/,
  );
});

test("commercial MCP mutations require the exact displayed confirmation", () => {
  assert.doesNotThrow(() =>
    assertExactConfirmation(
      "CONFIRM publish_route research-report",
      "CONFIRM publish_route research-report",
    ),
  );
  assert.throws(
    () =>
      assertExactConfirmation(
        "confirm publish_route research-report",
        "CONFIRM publish_route research-report",
      ),
    /did not match/,
  );
});

test("diagnostics redact identity and credential material", () => {
  const message = safeDiagnosticMessage(
    `seller@example.test Bearer eyJabc.def.ghi apc2.key_123.secret 0x${"a".repeat(64)}`,
  );
  assert.equal(message.includes("seller@example.test"), false);
  assert.equal(message.includes("eyJabc"), false);
  assert.equal(message.includes("apc2."), false);
  assert.equal(message.includes(`0x${"a".repeat(64)}`), false);
});

test("seller rehearsal source does not call buyer commerce endpoints", async () => {
  const source = await readFile(
    new URL("./seller-onboarding-rehearsal.mjs", import.meta.url),
    "utf8",
  );
  assert.doesNotMatch(source, /purchase-intents|purchase-sessions|\/pay(?:["'`/])/u);
  assert.doesNotMatch(source, /env:\s*\{\s*\.\.\.process\.env/u);
});

test("signed discovery is verified against the matching ES256 JWKS key", () => {
  const { privateKey, publicKey } = generateKeyPairSync("ec", {
    namedCurve: "P-256",
  });
  const document = { schemaVersion: "agentpay.discovery.v1", products: [] };
  const payload = Buffer.from(
    'agentpay.discovery.v1\0{"products":[],"schemaVersion":"agentpay.discovery.v1"}',
    "utf8",
  );
  const signature = sign("sha256", payload, {
    key: privateKey,
    dsaEncoding: "ieee-p1363",
  }).toString("base64url");
  const jwk = publicKey.export({ format: "jwk" });
  assert.equal(
    verifySignedDiscovery(
      {
        document,
        signature: {
          alg: "ES256",
          kid: "discovery-key",
          canonicalization: "RFC8785",
          domainSeparator: "agentpay.discovery.v1",
          value: signature,
        },
      },
      { keys: [{ ...jwk, kid: "discovery-key", alg: "ES256", use: "sig" }] },
    ),
    true,
  );
});

test("sanitized evidence rejects secret-bearing fields and values", () => {
  const recorder = createEvidenceRecorder({
    commit: "abc123",
    apiOrigin: "https://api.agentpay.example",
    webOrigin: "https://agentpay.example",
  });
  recorder.pass("Seller created", { sellerId: "sel_123" });
  assert.throws(
    () => recorder.pass("Credential created", { projectKey: "apc2.key.secret" }),
    /secret-bearing evidence field/,
  );
  assert.throws(
    () => recorder.pass("Auth", { note: "Bearer eyJsecret" }),
    /secret-like evidence value/,
  );
  assert.deepEqual(recorder.document().steps[0], {
    name: "Seller created",
    status: "passed",
    sellerId: "sel_123",
  });
});
