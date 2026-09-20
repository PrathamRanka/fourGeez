import assert from "node:assert/strict";
import test from "node:test";

import {
  DynamoDbFulfillmentStore,
  DynamoDbWebhookReplayStore,
} from "../dist/dynamodb.js";

const now = new Date("2026-09-20T12:00:00Z");

test("DynamoDB webhook replay claims are shared and TTL bounded", async () => {
  const client = new FakeDynamoDbClient();
  const options = {
    client,
    tableName: "seller-state",
    sellerId: "sel_test",
    now: () => now,
    retentionSeconds: 600,
  };
  const first = new DynamoDbWebhookReplayStore(options);
  const second = new DynamoDbWebhookReplayStore(options);

  assert.equal(await first.claim("evt_sensitive", new Date(0)), true);
  assert.equal(await second.claim("evt_sensitive", new Date(0)), false);
  const [item] = [...client.items.values()];
  assert.match(item.SK, /^REPLAY#webhook#[a-f0-9]{64}$/u);
  assert.equal(item.SK.includes("evt_sensitive"), false);
  assert.equal(item.expiresAt, Math.floor(now.getTime() / 1000) + 600);
});

test("DynamoDB fulfillment admits one claimant and replays its result", async () => {
  const client = new FakeDynamoDbClient();
  const options = {
    client,
    tableName: "seller-state",
    sellerId: "sel_test",
    now: () => now,
  };
  const first = new DynamoDbFulfillmentStore(options);
  const second = new DynamoDbFulfillmentStore(options);

  assert.deepEqual(await first.begin("txn_once"), { status: "started" });
  assert.deepEqual(await second.begin("txn_once"), { status: "in_progress" });
  await first.complete("txn_once", { deliveryId: "delivery-1" });
  await assert.rejects(
    first.complete("txn_once", { deliveryId: "delivery-2" }),
    { name: "ConditionalCheckFailedException" },
  );
  assert.deepEqual(await second.begin("txn_once"), {
    status: "completed",
    result: { deliveryId: "delivery-1" },
  });

  const [item] = [...client.items.values()];
  assert.equal(item.SK, "FULFILLMENT#txn_once");
  assert.equal(item.status, "completed");
  assert.equal("expiresAt" in item, false);
});

test("DynamoDB store errors fail replay claims closed", async () => {
  const store = new DynamoDbWebhookReplayStore({
    client: {
      async send() {
        throw new Error("DynamoDB unavailable");
      },
    },
    tableName: "seller-state",
    sellerId: "sel_test",
    now: () => now,
  });

  await assert.rejects(store.claim("evt_failure", now), /unavailable/u);
});

test("DynamoDB fulfillment releases only an in-progress claim", async () => {
  const client = new FakeDynamoDbClient();
  const store = new DynamoDbFulfillmentStore({
    client,
    tableName: "seller-state",
    sellerId: "sel_test",
    now: () => now,
  });

  await store.begin("txn_retry");
  await store.release("txn_retry");
  assert.deepEqual(await store.begin("txn_retry"), { status: "started" });
});

class FakeDynamoDbClient {
  items = new Map();

  async send(command) {
    const input = command.input;
    if (command.constructor.name === "PutCommand") {
      const key = keyOf(input.Item);
      if (this.items.has(key)) throw conditionalFailure();
      this.items.set(key, structuredClone(input.Item));
      return {};
    }
    if (command.constructor.name === "GetCommand") {
      return { Item: structuredClone(this.items.get(keyOf(input.Key))) };
    }
    if (command.constructor.name === "UpdateCommand") {
      const key = keyOf(input.Key);
      const item = this.items.get(key);
      if (!item || item.status !== "in_progress") throw conditionalFailure();
      item.status = input.ExpressionAttributeValues[":completed"];
      item.result = input.ExpressionAttributeValues[":result"];
      item.updatedAt = input.ExpressionAttributeValues[":updatedAt"];
      return {};
    }
    if (command.constructor.name === "DeleteCommand") {
      const key = keyOf(input.Key);
      const item = this.items.get(key);
      if (!item || item.status !== "in_progress") throw conditionalFailure();
      this.items.delete(key);
      return {};
    }
    throw new Error(`unsupported command ${command.constructor.name}`);
  }
}

function keyOf(value) {
  return `${value.PK}|${value.SK}`;
}

function conditionalFailure() {
  const error = new Error("conditional failure");
  error.name = "ConditionalCheckFailedException";
  return error;
}
