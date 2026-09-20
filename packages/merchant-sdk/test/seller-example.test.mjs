import assert from "node:assert/strict";
import test from "node:test";

import { createSellerStateStores } from "../../../examples/merchant-sdk-node/state-stores.mjs";

test("seller example defaults to shared DynamoDB state", () => {
  const client = { async send() {} };
  const stores = createSellerStateStores(
    { AGENTPAY_DYNAMODB_TABLE: "seller-state" },
    "sel_test",
    () => client,
  );

  assert.equal(stores.mode, "dynamodb");
  assert.match(stores.executionReplayStore.constructor.name, /^DynamoDb/u);
  assert.match(stores.fulfillmentStore.constructor.name, /^DynamoDb/u);
  assert.match(stores.webhookReplayStore.constructor.name, /^DynamoDb/u);
});

test("seller example permits memory state only when explicitly local", () => {
  const stores = createSellerStateStores(
    { AGENTPAY_STATE_MODE: "local-memory" },
    "sel_test",
  );

  assert.equal(stores.mode, "local-memory");
  assert.match(stores.executionReplayStore.constructor.name, /^Memory/u);
  assert.match(stores.fulfillmentStore.constructor.name, /^Memory/u);
  assert.match(stores.webhookReplayStore.constructor.name, /^Memory/u);
});
