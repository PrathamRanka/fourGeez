import { MerchantSdkError } from "./errors.js";

export type FulfillmentBeginResult<TResult> =
  | { status: "started" }
  | { status: "in_progress" }
  | { status: "completed"; result: TResult };

export interface FulfillmentStore<TResult> {
  begin(transactionId: string): Promise<FulfillmentBeginResult<TResult>>;
  complete(transactionId: string, result: TResult): Promise<void>;
  release(transactionId: string): Promise<void>;
}

export interface FulfillOnceOptions<TResult> {
  transactionId: string;
  store: FulfillmentStore<TResult>;
  execute: () => Promise<TResult>;
}

export interface FulfillmentOutcome<TResult> {
  disposition: "executed" | "replayed";
  result: TResult;
}

type MemoryFulfillmentState<TResult> =
  { status: "in_progress" } | { status: "completed"; result: TResult };

export class MemoryFulfillmentStore<
  TResult = unknown,
> implements FulfillmentStore<TResult> {
  readonly #records = new Map<string, MemoryFulfillmentState<TResult>>();

  async begin(transactionId: string): Promise<FulfillmentBeginResult<TResult>> {
    const current = this.#records.get(transactionId);
    if (current?.status === "completed") {
      return { status: "completed", result: current.result };
    }
    if (current?.status === "in_progress") {
      return { status: "in_progress" };
    }
    this.#records.set(transactionId, { status: "in_progress" });
    return { status: "started" };
  }

  async complete(transactionId: string, result: TResult): Promise<void> {
    if (this.#records.get(transactionId)?.status !== "in_progress") {
      throw new Error("fulfillment was not claimed");
    }
    this.#records.set(transactionId, { status: "completed", result });
  }

  async release(transactionId: string): Promise<void> {
    if (this.#records.get(transactionId)?.status === "in_progress") {
      this.#records.delete(transactionId);
    }
  }
}

export async function fulfillOnce<TResult>(
  options: FulfillOnceOptions<TResult>,
): Promise<FulfillmentOutcome<TResult>> {
  let beginResult: FulfillmentBeginResult<TResult>;
  try {
    beginResult = await options.store.begin(options.transactionId);
  } catch {
    throw new MerchantSdkError("dependency_unavailable", 503);
  }

  if (beginResult.status === "completed") {
    return { disposition: "replayed", result: beginResult.result };
  }
  if (beginResult.status === "in_progress") {
    throw new MerchantSdkError("fulfillment_in_progress", 409);
  }

  let result: TResult;
  try {
    result = await options.execute();
  } catch (executionError) {
    try {
      await options.store.release(options.transactionId);
    } catch {
      throw new MerchantSdkError("dependency_unavailable", 503);
    }
    throw executionError;
  }

  try {
    await options.store.complete(options.transactionId, result);
  } catch {
    throw new MerchantSdkError("dependency_unavailable", 503);
  }
  return { disposition: "executed", result };
}
