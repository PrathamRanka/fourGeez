import { createHash } from "node:crypto";

import { PutCommand, type DynamoDBDocumentClient } from "@aws-sdk/lib-dynamodb";

import type { ExecutionReplayStore, ReplayStore } from "./index.js";

const DEFAULT_REPLAY_RETENTION_SECONDS = 10 * 60;
const MINIMUM_REPLAY_RETENTION_SECONDS = 5 * 60;

export interface DynamoDbReplayStoreOptions {
  client: Pick<DynamoDBDocumentClient, "send">;
  tableName: string;
  sellerId: string;
  retentionSeconds?: number;
  now?: () => Date;
}

export class DynamoDbExecutionReplayStore implements ExecutionReplayStore {
  readonly #client: Pick<DynamoDBDocumentClient, "send">;
  readonly #tableName: string;
  readonly #partitionKey: string;
  readonly #now: () => Date;

  constructor(options: DynamoDbReplayStoreOptions) {
    const configuration = validateOptions(options);
    this.#client = options.client;
    this.#tableName = configuration.tableName;
    this.#partitionKey = configuration.partitionKey;
    this.#now = options.now ?? (() => new Date());
  }

  async claim(jti: string, expiresAt: Date): Promise<boolean> {
    return claimIdentifier({
      client: this.#client,
      tableName: this.#tableName,
      partitionKey: this.#partitionKey,
      kind: "execution",
      identifier: jti,
      expiresAt: epochSeconds(expiresAt),
      createdAt: this.#now().toISOString(),
    });
  }
}

export class DynamoDbReplayStore implements ReplayStore {
  readonly #client: Pick<DynamoDBDocumentClient, "send">;
  readonly #tableName: string;
  readonly #partitionKey: string;
  readonly #retentionSeconds: number;
  readonly #now: () => Date;

  constructor(options: DynamoDbReplayStoreOptions) {
    const configuration = validateOptions(options);
    this.#client = options.client;
    this.#tableName = configuration.tableName;
    this.#partitionKey = configuration.partitionKey;
    this.#retentionSeconds = configuration.retentionSeconds;
    this.#now = options.now ?? (() => new Date());
  }

  async claim(transactionId: string): Promise<boolean> {
    return claimIdentifier({
      client: this.#client,
      tableName: this.#tableName,
      partitionKey: this.#partitionKey,
      kind: "legacy_request",
      identifier: transactionId,
      expiresAt: epochSeconds(this.#now()) + this.#retentionSeconds,
      createdAt: this.#now().toISOString(),
    });
  }
}

interface ClaimIdentifierOptions {
  client: Pick<DynamoDBDocumentClient, "send">;
  tableName: string;
  partitionKey: string;
  kind: "execution" | "legacy_request";
  identifier: string;
  expiresAt: number;
  createdAt: string;
}

async function claimIdentifier(
  options: ClaimIdentifierOptions,
): Promise<boolean> {
  if (!options.identifier) throw new Error("replay identifier is required");
  try {
    await options.client.send(
      new PutCommand({
        TableName: options.tableName,
        Item: {
          PK: options.partitionKey,
          SK: replaySortKey(options.kind, options.identifier),
          recordType: `${options.kind}_replay`,
          createdAt: options.createdAt,
          expiresAt: options.expiresAt,
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

function validateOptions(options: DynamoDbReplayStoreOptions) {
  const tableName = options.tableName.trim();
  const sellerId = options.sellerId.trim();
  const retentionSeconds =
    options.retentionSeconds ?? DEFAULT_REPLAY_RETENTION_SECONDS;
  if (!tableName || !/^sel_[A-Za-z0-9_-]+$/u.test(sellerId)) {
    throw new Error("valid DynamoDB tableName and sellerId are required");
  }
  if (
    !Number.isSafeInteger(retentionSeconds) ||
    retentionSeconds < MINIMUM_REPLAY_RETENTION_SECONDS
  ) {
    throw new Error("replay retention must be at least 300 seconds");
  }
  return {
    tableName,
    partitionKey: `AGENTPAY_SELLER#${sellerId}`,
    retentionSeconds,
  };
}

function replaySortKey(kind: string, identifier: string): string {
  const digest = createHash("sha256")
    .update(`agentpay.seller-runtime.${kind}.v1\0${identifier}`)
    .digest("hex");
  return `REPLAY#${kind}#${digest}`;
}

function epochSeconds(value: Date): number {
  const milliseconds = value.getTime();
  if (!Number.isFinite(milliseconds))
    throw new Error("valid expiry is required");
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
