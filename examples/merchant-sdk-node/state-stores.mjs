import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import { DynamoDBDocumentClient } from "@aws-sdk/lib-dynamodb";
import {
  MemoryExecutionReplayStore,
  MemoryFulfillmentStore,
  MemoryWebhookReplayStore,
} from "../../packages/merchant-sdk/dist/index.js";
import {
  DynamoDbExecutionReplayStore,
  DynamoDbFulfillmentStore,
  DynamoDbWebhookReplayStore,
} from "../../packages/merchant-sdk/dist/dynamodb.js";

export function createSellerStateStores(
  environment,
  sellerId,
  createDocumentClient = defaultDocumentClient,
) {
  const mode = environment.AGENTPAY_STATE_MODE?.trim() || "dynamodb";
  if (mode === "local-memory") {
    return {
      mode,
      executionReplayStore: new MemoryExecutionReplayStore(),
      fulfillmentStore: new MemoryFulfillmentStore(),
      webhookReplayStore: new MemoryWebhookReplayStore(),
    };
  }
  if (mode !== "dynamodb") {
    throw new Error("AGENTPAY_STATE_MODE must be dynamodb or local-memory");
  }

  const tableName = requiredEnvironment(environment, "AGENTPAY_DYNAMODB_TABLE");
  const client = createDocumentClient(environment);
  const options = { client, tableName, sellerId };
  return {
    mode,
    executionReplayStore: new DynamoDbExecutionReplayStore(options),
    fulfillmentStore: new DynamoDbFulfillmentStore(options),
    webhookReplayStore: new DynamoDbWebhookReplayStore(options),
  };
}

function defaultDocumentClient(environment) {
  const region = requiredEnvironment(environment, "AWS_REGION");
  const endpoint = environment.AGENTPAY_DYNAMODB_ENDPOINT?.trim();
  return DynamoDBDocumentClient.from(
    new DynamoDBClient({ region, ...(endpoint ? { endpoint } : {}) }),
    { marshallOptions: { removeUndefinedValues: false } },
  );
}

function requiredEnvironment(environment, name) {
  const value = environment[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
