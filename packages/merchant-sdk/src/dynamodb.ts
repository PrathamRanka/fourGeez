import { createHash } from "node:crypto";

import {
  DeleteCommand,
  GetCommand,
  PutCommand,
  UpdateCommand,
  type DynamoDBDocumentClient,
} from "@aws-sdk/lib-dynamodb";
export {
  DynamoDbExecutionReplayStore,
  DynamoDbReplayStore,
} from "@agentpay/verify-node/dynamodb";

import type {
  FulfillmentBeginResult,
  FulfillmentStore,
} from "./fulfillment.js";
import type { WebhookReplayStore } from "./webhooks.js";

const DEFAULT_REPLAY_RETENTION_SECONDS = 10 * 60;
const MINIMUM_REPLAY_RETENTION_SECONDS = 5 * 60;
const DEFAULT_MAXIMUM_RESULT_BYTES = 256 * 1024;

interface DynamoDbStoreOptions {
  client: Pick<DynamoDBDocumentClient, "send">;
  tableName: string;
  sellerId: string;
  now?: () => Date;
}

export interface DynamoDbWebhookReplayStoreOptions extends DynamoDbStoreOptions {
  retentionSeconds?: number;
}

export interface DynamoDbFulfillmentStoreOptions extends DynamoDbStoreOptions {
  maximumResultBytes?: number;
}

export class DynamoDbWebhookReplayStore implements WebhookReplayStore {
  readonly #client: Pick<DynamoDBDocumentClient, "send">;
  readonly #tableName: string;
  readonly #partitionKey: string;
  readonly #retentionSeconds: number;
  readonly #now: () => Date;

  constructor(options: DynamoDbWebhookReplayStoreOptions) {
    const configuration = validateOptions(options);
    const retentionSeconds =
      options.retentionSeconds ?? DEFAULT_REPLAY_RETENTION_SECONDS;
    if (
      !Number.isSafeInteger(retentionSeconds) ||
      retentionSeconds < MINIMUM_REPLAY_RETENTION_SECONDS
    ) {
      throw new Error("replay retention must be at least 300 seconds");
    }
    this.#client = options.client;
    this.#tableName = configuration.tableName;
    this.#partitionKey = configuration.partitionKey;
    this.#retentionSeconds = retentionSeconds;
    this.#now = options.now ?? (() => new Date());
  }

  async claim(eventId: string): Promise<boolean> {
    if (!eventId) throw new Error("webhook event ID is required");
    const now = this.#now();
    const digest = createHash("sha256")
      .update(`agentpay.seller-runtime.webhook.v1\0${eventId}`)
      .digest("hex");
    try {
      await this.#client.send(
        new PutCommand({
          TableName: this.#tableName,
          Item: {
            PK: this.#partitionKey,
            SK: `REPLAY#webhook#${digest}`,
            recordType: "webhook_replay",
            createdAt: now.toISOString(),
            expiresAt: epochSeconds(now) + this.#retentionSeconds,
          },
          ConditionExpression:
            "attribute_not_exists(PK) AND attribute_not_exists(SK)",
        }),
      );
      return true;
    } catch (error) {
      if (isConditionalCheckFailure(error)) return false;
      throw error;
    }
  }
}

export class DynamoDbFulfillmentStore<
  TResult = unknown,
> implements FulfillmentStore<TResult> {
  readonly #client: Pick<DynamoDBDocumentClient, "send">;
  readonly #tableName: string;
  readonly #partitionKey: string;
  readonly #maximumResultBytes: number;
  readonly #now: () => Date;

  constructor(options: DynamoDbFulfillmentStoreOptions) {
    const configuration = validateOptions(options);
    const maximumResultBytes =
      options.maximumResultBytes ?? DEFAULT_MAXIMUM_RESULT_BYTES;
    if (!Number.isSafeInteger(maximumResultBytes) || maximumResultBytes <= 0) {
      throw new Error("maximumResultBytes must be a positive integer");
    }
    this.#client = options.client;
    this.#tableName = configuration.tableName;
    this.#partitionKey = configuration.partitionKey;
    this.#maximumResultBytes = maximumResultBytes;
    this.#now = options.now ?? (() => new Date());
  }

  async begin(transactionId: string): Promise<FulfillmentBeginResult<TResult>> {
    const key = this.#key(transactionId);
    const timestamp = this.#now().toISOString();
    try {
      await this.#client.send(
        new PutCommand({
          TableName: this.#tableName,
          Item: {
            ...key,
            recordType: "fulfillment",
            status: "in_progress",
            createdAt: timestamp,
            updatedAt: timestamp,
          },
          ConditionExpression:
            "attribute_not_exists(PK) AND attribute_not_exists(SK)",
        }),
      );
      return { status: "started" };
    } catch (error) {
      if (!isConditionalCheckFailure(error)) throw error;
    }

    const output = await this.#client.send(
      new GetCommand({
        TableName: this.#tableName,
        Key: key,
        ConsistentRead: true,
      }),
    );
    const item = output.Item;
    if (!item || item.recordType !== "fulfillment") {
      throw new Error("fulfillment claim could not be loaded");
    }
    if (item.status === "in_progress") return { status: "in_progress" };
    if (item.status !== "completed" || typeof item.result !== "string") {
      throw new Error("fulfillment record is invalid");
    }
    return { status: "completed", result: JSON.parse(item.result) as TResult };
  }

  async complete(transactionId: string, result: TResult): Promise<void> {
    const resultJson = serializeResult(result, this.#maximumResultBytes);
    await this.#client.send(
      new UpdateCommand({
        TableName: this.#tableName,
        Key: this.#key(transactionId),
        UpdateExpression:
          "SET #status = :completed, #result = :result, #updatedAt = :updatedAt",
        ConditionExpression:
          "#recordType = :recordType AND #status = :inProgress",
        ExpressionAttributeNames: {
          "#recordType": "recordType",
          "#status": "status",
          "#result": "result",
          "#updatedAt": "updatedAt",
        },
        ExpressionAttributeValues: {
          ":recordType": "fulfillment",
          ":inProgress": "in_progress",
          ":completed": "completed",
          ":result": resultJson,
          ":updatedAt": this.#now().toISOString(),
        },
      }),
    );
  }

  async release(transactionId: string): Promise<void> {
    try {
      await this.#client.send(
        new DeleteCommand({
          TableName: this.#tableName,
          Key: this.#key(transactionId),
          ConditionExpression:
            "#recordType = :recordType AND #status = :inProgress",
          ExpressionAttributeNames: {
            "#recordType": "recordType",
            "#status": "status",
          },
          ExpressionAttributeValues: {
            ":recordType": "fulfillment",
            ":inProgress": "in_progress",
          },
        }),
      );
    } catch (error) {
      if (!isConditionalCheckFailure(error)) throw error;
    }
  }

  #key(transactionId: string) {
    if (!/^txn_[A-Za-z0-9_-]+$/u.test(transactionId)) {
      throw new Error("valid AgentPay transactionId is required");
    }
    return {
      PK: this.#partitionKey,
      SK: `FULFILLMENT#${transactionId}`,
    };
  }
}

function validateOptions(options: DynamoDbStoreOptions) {
  const tableName = options.tableName.trim();
  const sellerId = options.sellerId.trim();
  if (!tableName || !/^sel_[A-Za-z0-9_-]+$/u.test(sellerId)) {
    throw new Error("valid DynamoDB tableName and sellerId are required");
  }
  return { tableName, partitionKey: `AGENTPAY_SELLER#${sellerId}` };
}

function serializeResult(value: unknown, maximumBytes: number): string {
  let result: string | undefined;
  try {
    result = JSON.stringify(value);
  } catch {
    throw new Error("fulfillment result must be JSON serializable");
  }
  if (result === undefined) {
    throw new Error("fulfillment result must be JSON serializable");
  }
  if (Buffer.byteLength(result) > maximumBytes) {
    throw new Error("fulfillment result exceeds the configured size limit");
  }
  return result;
}

function epochSeconds(value: Date): number {
  const milliseconds = value.getTime();
  if (!Number.isFinite(milliseconds)) throw new Error("valid time is required");
  return Math.ceil(milliseconds / 1000);
}

function isConditionalCheckFailure(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "name" in error &&
    error.name === "ConditionalCheckFailedException"
  );
}
