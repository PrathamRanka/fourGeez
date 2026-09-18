"use server";

import type {
  CreateIntegrationCredentialInput,
  CreateStorefrontInput,
  CredentialCreated,
  IntegrationCredential,
  PaymentDestination,
  PreparePaymentDestinationInput,
  PreparedPaymentDestination,
  Seller,
  VerifyPaymentDestinationInput,
} from "@/features/onboarding/model";
import { requestAgentPay, type ActionResult } from "@/lib/agentpay-api";

// createStorefront creates the seller record that owns every later onboarding resource.
export async function createStorefront(
  input: CreateStorefrontInput,
): Promise<ActionResult<Seller>> {
  return requestAgentPay<Seller>("/v1/sellers", {
    method: "POST",
    body: input,
  });
}

// preparePaymentDestination creates a pending destination and its ownership challenge.
export async function preparePaymentDestination(
  input: PreparePaymentDestinationInput,
): Promise<ActionResult<PreparedPaymentDestination>> {
  const destinationResult = await requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations`,
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
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations/${encodeURIComponent(destinationResult.value.destinationId)}/ownership-challenges`,
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
  return requestAgentPay<PaymentDestination>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/payment-destinations/${encodeURIComponent(input.destinationId)}/verify`,
    {
      method: "POST",
      body: {
        challenge: input.challenge,
        signature: input.signature,
        confirmRotation: false,
      },
    },
  );
}

// createIntegrationCredential issues the project key once with minimum onboarding scopes.
export async function createIntegrationCredential(
  input: CreateIntegrationCredentialInput,
): Promise<ActionResult<CredentialCreated>> {
  return requestAgentPay<CredentialCreated>(
    `/v1/sellers/${encodeURIComponent(input.sellerId)}/integration-credentials`,
    {
      method: "POST",
      body: {
        label: "Primary coding agent",
        scopes: ["read", "configure", "validate", "publish"],
        expiresAt: null,
      },
    },
  );
}

// listOnboardingResources restores non-secret wallet and credential status.
export async function listOnboardingResources(sellerId: string): Promise<{
  paymentDestinations: PaymentDestination[];
  credentials: IntegrationCredential[];
}> {
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

  return {
    paymentDestinations: paymentResult.ok ? paymentResult.value.items : [],
    credentials: credentialResult.ok ? credentialResult.value.items : [],
  };
}
