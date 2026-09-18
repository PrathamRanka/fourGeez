import type { AuthResult, IdentityAdapter } from "@/features/auth/model";
import { createLocalIdentityAdapter } from "@/features/auth/server/local-identity-adapter";

type LocalIdentity = ReturnType<typeof createLocalIdentityAdapter>;

declare global {
  var agentPayLocalIdentity: LocalIdentity | undefined;
}

function unavailableResult(): AuthResult<never> {
  return {
    ok: false,
    code: "dependency_unavailable",
    error: "Seller authentication is not configured for this environment.",
  };
}

const unavailableIdentity: IdentityAdapter = {
  signUp: async () => unavailableResult(),
  verifyRegistration: async () => unavailableResult(),
  signIn: async () => unavailableResult(),
  beginRecovery: async () => unavailableResult(),
  completeRecovery: async () => unavailableResult(),
  validate: async () => unavailableResult(),
  revoke: async () => undefined,
};

// getIdentityAdapter prevents the local adapter from silently running in production.
export function getIdentityAdapter(): IdentityAdapter {
  const identityMode =
    process.env.AGENTPAY_IDENTITY_MODE ??
    (process.env.AGENTPAY_ENV === "local" || process.env.NODE_ENV === "test"
      ? "local"
      : "cognito");
  if (identityMode !== "local") {
    return unavailableIdentity;
  }
  globalThis.agentPayLocalIdentity ??= createLocalIdentityAdapter({
    sellerAccessToken: process.env.AGENTPAY_SELLER_BEARER_TOKEN ?? "",
  });
  return globalThis.agentPayLocalIdentity;
}

export function getLocalIdentityAdapter(): LocalIdentity | null {
  const identity = getIdentityAdapter();
  return "updatePrincipal" in identity ? (identity as LocalIdentity) : null;
}
