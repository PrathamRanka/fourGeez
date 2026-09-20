import assert from "node:assert/strict";
import test from "node:test";

import {
  DynamoDbExecutionReplayStore,
  DynamoDbReplayStore,
} from "../dist/dynamodb.js";

const now = new Date("2026-09-20T12:00:00Z");

test("DynamoDB execution replay claims are atomic across store instances", async () => {
  const client = new FakeDynamoDbClient();
  const options = { client, tableName: "seller-state", sellerId: "sel_test" };
  const first = new DynamoDbExecutionReplayStore(options);
  const second = new DynamoDbExecutionReplayStore(options);
  const expiresAt = new Date("2026-09-20T12:01:00Z");

  assert.equal(await first.claim("xec_sensitive", expiresAt), true);
  assert.equal(await second.claim("xec_sensitive", expiresAt), false);

  const [item] = [...client.items.values()];
  assert.equal(item.PK, "AGENTPAY_SELLER#sel_test");
  assert.match(item.SK, /^REPLAY#execution#[a-f0-9]{64}$/u);
  assert.equal(item.SK.includes("xec_sensitive"), false);
  assert.equal(item.expiresAt, Math.floor(expiresAt.getTime() / 1000));
});

test("DynamoDB legacy replay claims use accepting-server TTL", async () => {
  const client = new FakeDynamoDbClient();
  const store = new DynamoDbReplayStore({
    client,
    tableName: "seller-state",
    sellerId: "sel_test",
    now: () => now,
    retentionSeconds: 600,
  });

  assert.equal(await store.claim("txn_sensitive", new Date(0)), true);
  assert.equal(await store.claim("txn_sensitive", new Date(0)), false);
  const [item] = [...client.items.values()];
  assert.equal(item.expiresAt, Math.floor(now.getTime() / 1000) + 600);
  assert.equal(item.SK.includes("txn_sensitive"), false);
});

test("DynamoDB errors fail replay claims closed", async () => {
  const store = new DynamoDbExecutionReplayStore({
    client: {
      async send() {
        throw new Error("DynamoDB unavailable");
      },
    },
    tableName: "seller-state",
    sellerId: "sel_test",
  });

  await assert.rejects(
    store.claim("xec_failure", new Date("2026-09-20T12:01:00Z")),
    /unavailable/u,
  );
});

class FakeDynamoDbClient {
  items = new Map();

  async send(command) {
    assert.equal(command.constructor.name, "PutCommand");
    const item = command.input.Item;
    const key = `${item.PK}|${item.SK}`;
    if (this.items.has(key)) {
      const error = new Error("conditional failure");
      error.name = "ConditionalCheckFailedException";
      throw error;
    }
    this.items.set(key, structuredClone(item));
    return {};
  }
}
