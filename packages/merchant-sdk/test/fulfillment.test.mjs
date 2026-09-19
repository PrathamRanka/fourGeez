import assert from "node:assert/strict";
import test from "node:test";
import {
  fulfillOnce,
  MemoryFulfillmentStore,
  MerchantSdkError,
} from "../dist/index.js";

test("executes fulfillment once and returns the durable result on retry", async () => {
  const store = new MemoryFulfillmentStore();
  let executions = 0;
  const execute = async () => {
    executions += 1;
    return { deliveryId: "delivery-1" };
  };

  const first = await fulfillOnce({
    transactionId: "txn_once",
    store,
    execute,
  });
  const replay = await fulfillOnce({
    transactionId: "txn_once",
    store,
    execute,
  });

  assert.deepEqual(first, {
    disposition: "executed",
    result: { deliveryId: "delivery-1" },
  });
  assert.deepEqual(replay, {
    disposition: "replayed",
    result: { deliveryId: "delivery-1" },
  });
  assert.equal(executions, 1);
});

test("rejects an overlapping fulfillment attempt", async () => {
  const store = new MemoryFulfillmentStore();
  await store.begin("txn_busy");

  await assert.rejects(
    fulfillOnce({
      transactionId: "txn_busy",
      store,
      execute: async () => ({ ok: true }),
    }),
    (error) =>
      error instanceof MerchantSdkError &&
      error.code === "fulfillment_in_progress" &&
      error.status === 409,
  );
});

test("releases a failed business attempt so it can be retried", async () => {
  const store = new MemoryFulfillmentStore();
  await assert.rejects(
    fulfillOnce({
      transactionId: "txn_retry",
      store,
      execute: async () => {
        throw new Error("seller failed");
      },
    }),
    /seller failed/,
  );

  const result = await fulfillOnce({
    transactionId: "txn_retry",
    store,
    execute: async () => ({ ok: true }),
  });
  assert.equal(result.disposition, "executed");
});

test("does not release an uncertain completion-store failure", async () => {
  const state = { inProgress: false };
  const store = {
    async begin() {
      if (state.inProgress) return { status: "in_progress" };
      state.inProgress = true;
      return { status: "started" };
    },
    async complete() {
      throw new Error("durability unavailable");
    },
    async release() {
      state.inProgress = false;
    },
  };

  await assert.rejects(
    fulfillOnce({
      transactionId: "txn_uncertain",
      store,
      execute: async () => ({ ok: true }),
    }),
    (error) =>
      error instanceof MerchantSdkError &&
      error.code === "dependency_unavailable",
  );
  assert.equal(state.inProgress, true);
});
