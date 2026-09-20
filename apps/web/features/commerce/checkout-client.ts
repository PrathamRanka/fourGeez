import type { PaymentRecoveryAction } from "@/features/commerce/x402-wallet";

export type CheckoutRequestResult<Value> =
  | { ok: true; value: Value }
  | {
      ok: false;
      error: string;
      retryable: boolean;
      recoveryAction?: PaymentRecoveryAction;
    };

// requestCheckout calls the same-origin commerce gateway and preserves bounded recovery guidance.
export async function requestCheckout<Value>(
  path: string,
  body: unknown,
): Promise<CheckoutRequestResult<Value>> {
  try {
    const response = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const responseBody: unknown = await response.json();
    if (!response.ok) {
      const apiError =
        isRecord(responseBody) && isRecord(responseBody.error)
          ? responseBody.error
          : null;
      const recoveryAction =
        apiError &&
        isRecord(apiError.details) &&
        isPaymentRecoveryAction(apiError.details.recoveryAction)
          ? apiError.details.recoveryAction
          : apiError &&
              (apiError.code === "payment_expired" ||
                apiError.code === "route_contract_stale")
            ? "start_new_checkout"
            : undefined;
      return {
        ok: false,
        error:
          apiError && typeof apiError.message === "string"
            ? apiError.message
            : "AgentPay could not complete checkout.",
        retryable:
          recoveryAction !== undefined ||
          response.status >= 500 ||
          response.status === 429,
        recoveryAction,
      };
    }
    return { ok: true, value: responseBody as Value };
  } catch {
    return {
      ok: false,
      error:
        "Checkout is temporarily unavailable. Check the connection and retry.",
      retryable: true,
    };
  }
}

function isPaymentRecoveryAction(
  value: unknown,
): value is PaymentRecoveryAction {
  return (
    value === "connect_wallet" ||
    value === "switch_network" ||
    value === "sign_fresh_authorization" ||
    value === "retry_same_request" ||
    value === "retry_same_payment" ||
    value === "start_new_checkout"
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
