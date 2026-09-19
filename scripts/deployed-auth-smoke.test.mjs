import test from "node:test";
import assert from "node:assert/strict";
import {
  CookieJar,
  canonicalWebOrigin,
  generateSmokePassword,
  parseBoolean,
  requireSmokeConfiguration,
} from "./deployed-auth-smoke-lib.mjs";

test("deployed auth smoke is pinned to the canonical production origin", () => {
  assert.deepEqual(
    requireSmokeConfiguration({ AGENTPAY_SMOKE_EMAIL: "seller@example.com" }),
    {
      email: "seller@example.com",
      origin: canonicalWebOrigin,
      waitForExpiry: false,
      waitForRefresh: false,
    },
  );
  assert.throws(
    () =>
      requireSmokeConfiguration({
        AGENTPAY_SMOKE_EMAIL: "seller@example.com",
        AGENTPAY_SMOKE_ORIGIN: "https://preview.example.com",
      }),
    /pinned/,
  );
});

test("deployed auth smoke requires a private test recipient", () => {
  assert.throws(() => requireSmokeConfiguration({}), /AGENTPAY_SMOKE_EMAIL/);
});

test("generated smoke passwords satisfy the Cognito minimum without fixed credentials", () => {
  const first = generateSmokePassword();
  const second = generateSmokePassword();
  assert.ok(first.length >= 12);
  assert.notEqual(first, second);
});

test("cookie jar applies rotations and deletions", () => {
  const jar = new CookieJar();
  jar.capture({
    getSetCookie: () => [
      "__Host-agentpay_seller_csrf=first; Secure; SameSite=Strict",
      "__Host-agentpay_seller_session=sealed; HttpOnly; Secure",
    ],
  });
  assert.equal(
    jar.header(),
    "__Host-agentpay_seller_csrf=first; __Host-agentpay_seller_session=sealed",
  );
  jar.capture({
    getSetCookie: () => [
      "__Host-agentpay_seller_csrf=second; Secure",
      "__Host-agentpay_seller_session=; Max-Age=0; Secure",
    ],
  });
  assert.equal(jar.value("__Host-agentpay_seller_csrf"), "second");
  assert.equal(jar.value("__Host-agentpay_seller_session"), undefined);
});

test("boolean smoke options accept explicit affirmative values only", () => {
  assert.equal(parseBoolean("true"), true);
  assert.equal(parseBoolean("YES"), true);
  assert.equal(parseBoolean("0"), false);
});
