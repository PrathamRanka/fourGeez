export type MerchantSdkErrorCode =
  | "binding_mismatch"
  | "dependency_unavailable"
  | "fulfillment_in_progress"
  | "invalid_capability"
  | "invalid_configuration"
  | "invalid_event"
  | "invalid_signature"
  | "replay"
  | "stale_request";

const safeMessages: Record<MerchantSdkErrorCode, string> = {
  binding_mismatch: "AgentPay request binding did not match",
  dependency_unavailable: "AgentPay verification dependency is unavailable",
  fulfillment_in_progress: "AgentPay fulfillment is already in progress",
  invalid_capability: "AgentPay execution capability is invalid",
  invalid_configuration: "AgentPay merchant SDK configuration is invalid",
  invalid_event: "AgentPay webhook event is invalid",
  invalid_signature: "AgentPay signature is invalid",
  replay: "AgentPay request has already been consumed",
  stale_request: "AgentPay request timestamp is outside the allowed window",
};

export class MerchantSdkError extends Error {
  constructor(
    public readonly code: MerchantSdkErrorCode,
    public readonly status: number,
  ) {
    super(safeMessages[code]);
    this.name = "MerchantSdkError";
  }
}
