import type { AuthResult, IdentityAdapter } from "@/features/auth/model";
import { createCognitoIdentityAdapter } from "@/features/auth/server/cognito-identity-adapter";
import { createLocalIdentityAdapter } from "@/features/auth/server/local-identity-adapter";

type LocalIdentity = ReturnType<typeof createLocalIdentityAdapter>;

declare global {
  var agentPayLocalIdentity: LocalIdentity | undefined;
  var agentPayCognitoIdentity:
    | {
        clientId: string;
        identity: IdentityAdapter;
        region: string;
      }
    | undefined;
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
  if (identityMode === "cognito") {
    const region = process.env.AWS_REGION?.trim() ?? "";
    const clientId =
      process.env.AGENTPAY_SELLER_USER_POOL_CLIENT_ID?.trim() ?? "";
    if (!region || !clientId) return unavailableIdentity;
    if (
      !globalThis.agentPayCognitoIdentity ||
      globalThis.agentPayCognitoIdentity.clientId !== clientId ||
      globalThis.agentPayCognitoIdentity.region !== region
    ) {
      globalThis.agentPayCognitoIdentity = {
        clientId,
        region,
        identity: createCognitoIdentityAdapter({ clientId, region }),
      };
    }
    return globalThis.agentPayCognitoIdentity.identity;
  }
  if (identityMode !== "local") {
    return unavailableIdentity;
  }
  const sellerTokenSigningSecret =
    process.env.AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET ?? "";
  const sellerAccessToken = process.env.AGENTPAY_SELLER_BEARER_TOKEN ?? "";
  if (!sellerTokenSigningSecret && !sellerAccessToken) {
    return unavailableIdentity;
  }
  const demoSellerId = process.env.AGENTPAY_DEMO_SELLER_ID ?? "";
  const demoEmail = process.env.AGENTPAY_LOCAL_DEMO_EMAIL ?? "";
  const demoPassword = process.env.AGENTPAY_LOCAL_DEMO_PASSWORD ?? "";
  const seedAccount =
    demoSellerId && demoEmail && demoPassword
      ? {
          email: demoEmail,
          name: process.env.AGENTPAY_LOCAL_DEMO_NAME ?? "Pratham",
          onboardingComplete: true,
          password: demoPassword,
          sellerId: demoSellerId,
          storefront: {
            sellerId: demoSellerId,
            name:
              process.env.AGENTPAY_LOCAL_DEMO_STOREFRONT_NAME ??
              "Northstar Research",
            slug:
              process.env.AGENTPAY_LOCAL_DEMO_STOREFRONT_SLUG ?? "demo-seller",
            upstreamBaseUrl:
              process.env.AGENTPAY_LOCAL_DEMO_UPSTREAM_BASE_URL ??
              "http://127.0.0.1:8090",
            status: "active" as const,
            version: 3,
          },
          subject: process.env.AGENTPAY_LOCAL_DEMO_SUBJECT ?? "local-seller",
        }
      : undefined;
  globalThis.agentPayLocalIdentity ??= createLocalIdentityAdapter({
    seedAccount,
    sellerTokenSigningSecret,
    sellerAccessToken,
  });
  return globalThis.agentPayLocalIdentity;
}

export function getLocalIdentityAdapter(): LocalIdentity | null {
  const identity = getIdentityAdapter();
  return "updatePrincipal" in identity ? (identity as LocalIdentity) : null;
}
