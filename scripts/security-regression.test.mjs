import test from "node:test";
import assert from "node:assert/strict";
import { localSecurityChecks, pendingExternalProofs } from "./security-regression.mjs";

test("security regression covers every issue 61-64 boundary", () => {
  const plan = localSecurityChecks.map((check) => `${check.name} ${check.args.join(" ")}`).join("\n");
  for (const expected of ["session expiry", "revocation", "suspension", "stale discovery", "replay", "cancellation", "receipt", "evidence denial", "webhook overlap rotation"]) {
    assert.match(plan, new RegExp(expected, "i"));
  }
  assert.ok(pendingExternalProofs.every((proof) => proof.length > 20));
});

test("security regression commands contain no secret-bearing arguments", () => {
  const serialized = JSON.stringify(localSecurityChecks);
  assert.doesNotMatch(serialized, /(authorization|cookie|private.?key|payment.?signature|wallet.?secret)=/i);
});
