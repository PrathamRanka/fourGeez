"use server";

import type {
  ActivateSellerServiceInput,
  CreateIntegrationCredentialInput,
  CreateStorefrontInput,
  CredentialCreated,
  IntegrationCredential,
  PaymentDestination,
  PreparePaymentDestinationInput,
  PreparedPaymentDestination,
  Seller,
  SellerOnboardingState,
  VerifyPaymentDestinationInput,
} from "@/features/onboarding/model";
import {
  authenticatedSellerId,
  sellerSessionRequired,
} from "@/features/auth/server/authorization";
import {
  getSellerSession,
  updateCurrentSellerPrincipal,
} from "@/features/auth/server/session";
import { requestAgentPay, type ActionResult } from "@/lib/agentpay-api";

// createStorefront creates the seller record that owns every later onboarding resource.
export async function createStorefront(
  input: CreateStorefrontInput,
): Promise<ActionResult<Seller>> {
  if (!(await getSellerSession())) return sellerSessionRequired();
  const result = await requestAgentPay<Seller>("/v1/sellers", {
    method: "POST",
    body: input,
  });
  if (result.ok) {
    await updateCurrentSellerPrincipal({
      sellerId: result.value.sellerId,
      onboardingComplete: false,
      storefront: result.value,
    });
  }
  return result;
}

// activateSellerService enables ES256 request verification without creating a shared seller secret.
export async function activateSellerService(
  input: ActivateSellerServiceInput,
): Promise<ActionResult<Seller>> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return sellerSessionRequired();
  const result = await requestAgentPay<Seller>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/service-activation`,
    { method: "POST", body: { expectedVersion: input.expectedVersion } },
  );
  if (result.ok) {
    await updateCurrentSellerPrincipal({
      sellerId,
      onboardingComplete: false,
      storefront: result.value,
    });
  }
  return result;
}

// preparePaymentDestination creates a pending destination and its ownership challenge.
export async function preparePaymentDestination(
  input: PreparePaymentDestinationInput,
): Promise<ActionResult<PreparedPaymentDestination>> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return sellerSessionRequired();
  const destinationResult = await requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/payment-destinations`,
    {
      method: "POST",
      body: {
        asset: input.asset,
        network: input.network,
        address: input.address,
      },
    },
  );
  if (!destinationResult.ok) {
    return destinationResult;
  }

  const challengeResult = await requestAgentPay<{ challenge: string }>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/payment-destinations/${encodeURIComponent(destinationResult.value.destinationId)}/ownership-challenges`,
    { method: "POST", body: {} },
  );
  if (!challengeResult.ok) {
    return challengeResult;
  }

  return {
    ok: true,
    value: {
      destination: destinationResult.value,
      challenge: challengeResult.value.challenge,
    },
  };
}

// verifyPaymentDestination submits the browser-wallet proof and activates the destination.
export async function verifyPaymentDestination(
  input: VerifyPaymentDestinationInput,
): Promise<ActionResult<PaymentDestination>> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return sellerSessionRequired();
  const result = await requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/payment-destinations/${encodeURIComponent(input.destinationId)}/verify`,
    {
      method: "POST",
      body: {
        challenge: input.challenge,
        signature: input.signature,
        confirmRotation: false,
      },
    },
  );
  if (result.ok) await refreshOnboardingCompletion(sellerId);
  return result;
}

// createIntegrationCredential issues the project key once with minimum onboarding scopes.
export async function createIntegrationCredential(
  _input: CreateIntegrationCredentialInput,
): Promise<ActionResult<CredentialCreated>> {
  void _input;
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return sellerSessionRequired();
  const result = await requestAgentPay<CredentialCreated>(
    `/v1/sellers/${encodeURIComponent(sellerId)}/integration-credentials`,
    {
      method: "POST",
      body: {
        label: "Primary coding agent",
        scopes: ["read", "configure", "validate", "publish"],
        expiresAt: null,
      },
    },
  );
  if (result.ok) await refreshOnboardingCompletion(sellerId);
  return result;
}

// listOnboardingResources restores non-secret wallet and credential status.
export async function listOnboardingResources(): Promise<{
  paymentDestinations: PaymentDestination[];
  credentials: IntegrationCredential[];
  onboarding: SellerOnboardingState;
}> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) {
    return {
      paymentDestinations: [],
      credentials: [],
      onboarding: emptyOnboardingState(),
    };
  }
  const encodedSellerId = encodeURIComponent(sellerId);
  const [paymentResult, credentialResult, onboardingResult] = await Promise.all(
    [
      requestAgentPay<{ items: PaymentDestination[] }>(
        `/v1/sellers/${encodedSellerId}/payment-destinations`,
        { method: "GET" },
      ),
      requestAgentPay<{ items: IntegrationCredential[] }>(
        `/v1/sellers/${encodedSellerId}/integration-credentials`,
        { method: "GET" },
      ),
      requestAgentPay<SellerOnboardingState>("/v1/me/onboarding", {
        method: "GET",
      }),
    ],
  );

  return {
    paymentDestinations: paymentResult.ok ? paymentResult.value.items : [],
    credentials: credentialResult.ok ? credentialResult.value.items : [],
    onboarding: onboardingResult.ok
      ? onboardingResult.value
      : emptyOnboardingState(sellerId),
  };
}

function emptyOnboardingState(
  sellerId: string | null = null,
): SellerOnboardingState {
  return {
    sellerId,
    complete: false,
    currentStep: "account_verified",
    steps: [],
    publication: { allowed: false, blockers: [] },
    version: 0,
  };
}

async function refreshOnboardingCompletion(sellerId: string): Promise<void> {
  const encodedSellerId = encodeURIComponent(sellerId);
  const [paymentResult, credentialResult] = await Promise.all([
    requestAgentPay<{ items: PaymentDestination[] }>(
      `/v1/sellers/${encodedSellerId}/payment-destinations`,
      { method: "GET" },
    ),
    requestAgentPay<{ items: IntegrationCredential[] }>(
      `/v1/sellers/${encodedSellerId}/integration-credentials`,
      { method: "GET" },
    ),
  ]);
  const paymentDestinations =
    paymentResult.ok && Array.isArray(paymentResult.value.items)
      ? paymentResult.value.items
      : [];
  const credentials =
    credentialResult.ok && Array.isArray(credentialResult.value.items)
      ? credentialResult.value.items
      : [];
  const onboardingComplete =
    paymentDestinations.some(
      (destination) => destination.status === "active",
    ) && credentials.some((credential) => !credential.revokedAt);
  if (onboardingComplete) {
    await updateCurrentSellerPrincipal({ sellerId, onboardingComplete: true });
  }
}
