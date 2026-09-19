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
  SellerTestPurchaseVerification,
  TestPurchaseRoute,
  VerifySellerTestPurchaseInput,
  VerifyPaymentDestinationInput,
} from "@/features/onboarding/model";
import type { PurchaseReceipt } from "@/features/commerce/model";
import type {
  Transaction,
  TransactionDetailSnapshot,
} from "@/features/transactions/model";
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
  testableRoutes: TestPurchaseRoute[];
  onboarding: SellerOnboardingState;
}> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) {
    return {
      paymentDestinations: [],
      credentials: [],
      testableRoutes: [],
      onboarding: emptyOnboardingState(),
    };
  }
  const encodedSellerId = encodeURIComponent(sellerId);
  const [paymentResult, credentialResult, routeResult, onboardingResult] =
    await Promise.all([
      requestAgentPay<{ items: PaymentDestination[] }>(
        `/v1/sellers/${encodedSellerId}/payment-destinations`,
        { method: "GET" },
      ),
      requestAgentPay<{ items: IntegrationCredential[] }>(
        `/v1/sellers/${encodedSellerId}/integration-credentials`,
        { method: "GET" },
      ),
      requestAgentPay<{ items: TestPurchaseRoute[] }>(
        `/v1/sellers/${encodedSellerId}/routes`,
        { method: "GET" },
      ),
      requestAgentPay<SellerOnboardingState>("/v1/me/onboarding", {
        method: "GET",
      }),
    ]);

  return {
    paymentDestinations: paymentResult.ok ? paymentResult.value.items : [],
    credentials: credentialResult.ok ? credentialResult.value.items : [],
    testableRoutes: routeResult.ok
      ? routeResult.value.items.filter(
          (route) =>
            route.lifecycleStatus === "published" &&
            route.enabled &&
            route.asset === "USDC" &&
            route.network === "eip155:84532",
        )
      : [],
    onboarding: onboardingResult.ok
      ? onboardingResult.value
      : emptyOnboardingState(sellerId),
  };
}

// verifySellerTestPurchase fails closed until authoritative commerce records agree.
export async function verifySellerTestPurchase(
  input: VerifySellerTestPurchaseInput,
): Promise<ActionResult<SellerTestPurchaseVerification>> {
  const sellerId = await authenticatedSellerId();
  if (!sellerId) return sellerSessionRequired();
  const encodedTransactionId = encodeURIComponent(input.transactionId);
  const encodedSellerId = encodeURIComponent(sellerId);
  const [detailResult, receiptResult, listResult] = await Promise.all([
    requestAgentPay<
      Pick<TransactionDetailSnapshot, "transaction" | "evidence">
    >(`/v1/transactions/${encodedTransactionId}`, { method: "GET" }),
    requestAgentPay<PurchaseReceipt>(
      `/v1/transactions/${encodedTransactionId}/receipt`,
      { method: "GET" },
    ),
    requestAgentPay<{ items: Transaction[] }>(
      `/v1/sellers/${encodedSellerId}/transactions?limit=50`,
      { method: "GET" },
    ),
  ]);
  const failedResult = [detailResult, receiptResult, listResult].find(
    (result) => !result.ok,
  );
  if (failedResult && !failedResult.ok) return failedResult;
  if (!detailResult.ok || !receiptResult.ok || !listResult.ok) {
    return testPurchaseVerificationFailure();
  }
  const transaction = detailResult.value.transaction;
  const evidence = detailResult.value.evidence;
  const receipt = receiptResult.value;
  const dashboardOccurrenceCount = listResult.value.items.filter(
    (candidate) => candidate.transactionId === input.transactionId,
  ).length;
  const forwardingEventCount = evidence.events.filter(
    (event) => event.eventType === "proxy.forwarded",
  ).length;
  const successfulDeliveryEventCount = evidence.events.filter(
    (event) => event.eventType === "delivery.succeeded",
  ).length;
  const verified =
    transaction.transactionId === input.transactionId &&
    transaction.sellerId === sellerId &&
    transaction.status === "FULFILLED" &&
    transaction.commerceLifecycle.fulfillmentState === "succeeded" &&
    dashboardOccurrenceCount === 1 &&
    forwardingEventCount === 1 &&
    successfulDeliveryEventCount === 1 &&
    evidence.valid &&
    evidence.events.length > 0 &&
    receipt.transaction.transactionId === input.transactionId &&
    receipt.evidence.verified &&
    receipt.evidence.eventCount === evidence.events.length;
  if (!verified) return testPurchaseVerificationFailure();

  const onboardingResult = await requestAgentPay<SellerOnboardingState>(
    "/v1/me/onboarding/sandbox-purchases",
    { method: "POST", body: { transactionId: input.transactionId } },
  );
  if (!onboardingResult.ok) return onboardingResult;
  return {
    ok: true,
    value: {
      transactionId: input.transactionId,
      dashboardOccurrenceCount: 1,
      evidenceEventCount: evidence.events.length,
      evidenceValid: true,
      fulfillmentExactlyOnce: true,
      receiptAvailable: true,
      onboarding: onboardingResult.value,
    },
  };
}

function testPurchaseVerificationFailure(): ActionResult<never> {
  return {
    ok: false,
    code: "test_purchase_unverified",
    status: 409,
    error:
      "The transaction is not yet fulfilled, evidenced, receipted, and reconciled exactly once. Retry verification without starting a second payment.",
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
