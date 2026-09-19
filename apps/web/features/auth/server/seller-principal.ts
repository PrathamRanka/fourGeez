import type {
  AuthResult,
  IdentityAuthentication,
  SellerPrincipal,
} from "@/features/auth/model";

const identityRequestTimeoutMilliseconds = 10_000;
const maximumIdentityResponseBytes = 65_536;

type SellerResponse = NonNullable<SellerPrincipal["storefront"]>;
type OnboardingResponse = {
  sellerId: string | null;
  complete: boolean;
};

async function readBoundedJSON(response: Response): Promise<unknown> {
  const contentLength = Number(response.headers.get("Content-Length"));
  if (
    Number.isFinite(contentLength) &&
    contentLength > maximumIdentityResponseBytes
  ) {
    throw new Error("Identity response exceeded the allowed size.");
  }
  const body = await response.text();
  if (new TextEncoder().encode(body).byteLength > maximumIdentityResponseBytes) {
    throw new Error("Identity response exceeded the allowed size.");
  }
  return body ? JSON.parse(body) : {};
}

function isSellerResponse(value: unknown): value is SellerResponse {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const seller = value as SellerResponse;
  return (
    typeof seller.sellerId === "string" &&
    typeof seller.name === "string" &&
    typeof seller.slug === "string" &&
    typeof seller.upstreamBaseUrl === "string" &&
    ["active", "draft", "suspended"].includes(seller.status) &&
    Number.isInteger(seller.version)
  );
}

function isOnboardingResponse(value: unknown): value is OnboardingResponse {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const onboarding = value as OnboardingResponse;
  return (
    (onboarding.sellerId === null ||
      typeof onboarding.sellerId === "string") &&
    typeof onboarding.complete === "boolean"
  );
}

function unavailable(): AuthResult<never> {
  return {
    ok: false,
    code: "dependency_unavailable",
    error: "Seller workspace is temporarily unavailable.",
  };
}

export async function hydrateSellerPrincipal(
  authentication: IdentityAuthentication,
): Promise<AuthResult<IdentityAuthentication>> {
  const apiOrigin = process.env.AGENTPAY_API_ORIGIN?.trim().replace(/\/$/, "");
  if (!apiOrigin) return unavailable();
  const request = (path: string) =>
    fetch(`${apiOrigin}${path}`, {
      method: "GET",
      headers: { Authorization: `Bearer ${authentication.accessToken}` },
      cache: "no-store",
      signal: AbortSignal.timeout(identityRequestTimeoutMilliseconds),
    });
  try {
    const [sellerResponse, onboardingResponse] = await Promise.all([
      request("/v1/me/seller"),
      request("/v1/me/onboarding"),
    ]);
    if (sellerResponse.status === 401 || onboardingResponse.status === 401) {
      return {
        ok: false,
        code: "session_revoked",
        error: "Your session has ended. Sign in again.",
      };
    }
    if (sellerResponse.status === 404) {
      return {
        ok: true,
        value: {
          ...authentication,
          principal: {
            ...authentication.principal,
            sellerId: null,
            onboardingComplete: false,
            storefront: null,
          },
        },
      };
    }
    if (!sellerResponse.ok || !onboardingResponse.ok) return unavailable();
    const [sellerBody, onboardingBody] = await Promise.all([
      readBoundedJSON(sellerResponse),
      readBoundedJSON(onboardingResponse),
    ]);
    if (
      !isSellerResponse(sellerBody) ||
      !isOnboardingResponse(onboardingBody) ||
      onboardingBody.sellerId !== sellerBody.sellerId
    ) {
      return unavailable();
    }
    return {
      ok: true,
      value: {
        ...authentication,
        principal: {
          ...authentication.principal,
          sellerId: sellerBody.sellerId,
          onboardingComplete: onboardingBody.complete,
          storefront: sellerBody,
        },
      },
    };
  } catch {
    return unavailable();
  }
}
