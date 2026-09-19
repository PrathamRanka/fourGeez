"use client";

import {
  Check,
  Circle,
  CircleAlert,
  LoaderCircle,
  ReceiptText,
  RotateCcw,
  ShieldCheck,
} from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import { requestCheckout } from "@/features/commerce/checkout-client";
import type {
  CheckoutCompleteResult,
  CheckoutStartResult,
} from "@/features/commerce/model";
import {
  createX402PaymentSignature,
  decodePaymentRequired,
  detectWalletCompatibility,
  getInjectedWallet,
  type DecodedPaymentRequired,
} from "@/features/commerce/x402-wallet";
import type {
  SellerTestPurchaseVerification,
  TestPurchaseRoute,
  VerifySellerTestPurchaseInput,
} from "@/features/onboarding/model";
import type { ActionResult } from "@/lib/agentpay-api";
import { formatAtomicPrice } from "@/lib/money";
import styles from "./seller-test-purchase.module.css";

type CheckpointName =
  "intent" | "payment" | "fulfillment" | "evidence" | "reconciliation";
type CheckpointStatus = "pending" | "running" | "passed" | "failed";
type Checkpoint = {
  name: CheckpointName;
  label: string;
  detail: string;
  status: CheckpointStatus;
};

const initialCheckpoints: Checkpoint[] = [
  {
    name: "intent",
    label: "Intent created",
    detail: "Freeze the seller-approved quote and request hash.",
    status: "pending",
  },
  {
    name: "payment",
    label: "Payment verified",
    detail: "Use the payment mode returned by the x402 challenge.",
    status: "pending",
  },
  {
    name: "fulfillment",
    label: "Exactly-once fulfillment",
    detail: "Require one forwarding event and one successful delivery event.",
    status: "pending",
  },
  {
    name: "evidence",
    label: "Receipt and evidence verified",
    detail: "Require an authoritative receipt and valid evidence chain.",
    status: "pending",
  },
  {
    name: "reconciliation",
    label: "Dashboard reconciled",
    detail: "Require exactly one seller transaction record.",
    status: "pending",
  },
];

export function SellerTestPurchase({
  eligible,
  onVerified,
  route,
  sellerSlug,
  verifyPurchase,
}: {
  eligible: boolean;
  onVerified?: (verification: SellerTestPurchaseVerification) => void;
  route: TestPurchaseRoute | null;
  sellerSlug: string;
  verifyPurchase: (
    input: VerifySellerTestPurchaseInput,
  ) => Promise<ActionResult<SellerTestPurchaseVerification>>;
}) {
  const [checkpoints, setCheckpoints] = useState(initialCheckpoints);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [paymentMode, setPaymentMode] = useState<"mock" | "x402" | null>(null);
  const [transactionId, setTransactionId] = useState<string | null>(null);

  if (!eligible || !route) return null;
  const testRoute = route;

  async function runTestPurchase() {
    setRunning(true);
    setError(null);
    setPaymentMode(null);
    setTransactionId(null);
    setCheckpoints(withCheckpoint(initialCheckpoints, "intent", "running"));
    try {
      const requestBody = testRoute.method === "POST" ? "{}" : "";
      const startResult = await requestCheckout<CheckoutStartResult>(
        "/api/commerce/start",
        {
          channel: "browser",
          sellerSlug,
          productSlug: testRoute.productSlug,
          routeId: testRoute.routeId,
          maximumAmount: testRoute.amount,
          requestBody,
        },
      );
      if (!startResult.ok) throw new Error(startResult.error);
      setPaymentMode(startResult.value.paymentMode);
      setCheckpoints((current) =>
        withCheckpoint(
          withCheckpoint(current, "intent", "passed"),
          "payment",
          "running",
        ),
      );

      const challenge = decodePaymentRequired(
        startResult.value.paymentRequired,
      );
      assertTestPaymentTerms(
        startResult.value,
        challenge,
        testRoute,
        sellerSlug,
      );
      const paymentSignature =
        challenge.mode === "mock"
          ? "mock-approved-proof"
          : await createTestnetPaymentSignature(
              startResult.value.paymentRequired,
            );
      const completionBody = {
        channel: "browser" as const,
        sellerSlug,
        requestBody,
        purchaseIntent: startResult.value.purchaseIntent,
        paymentSignature,
      };
      const completion = await requestCheckout<CheckoutCompleteResult>(
        "/api/commerce/complete",
        completionBody,
      );
      if (!completion.ok) throw new Error(completion.error);
      setCheckpoints((current) =>
        withCheckpoint(
          withCheckpoint(current, "payment", "passed"),
          "fulfillment",
          "running",
        ),
      );

      const verification = await verifyPurchase({
        transactionId: completion.value.transactionId,
      });
      if (!verification.ok) throw new Error(verification.error);
      setTransactionId(verification.value.transactionId);
      setCheckpoints((current) =>
        withCheckpoint(
          withCheckpoint(
            withCheckpoint(
              withCheckpoint(current, "fulfillment", "passed"),
              "evidence",
              "passed",
            ),
            "reconciliation",
            "passed",
          ),
          "payment",
          "passed",
        ),
      );
      onVerified?.(verification.value);
    } catch (caughtError) {
      setCheckpoints((current) =>
        current.map((checkpoint) =>
          checkpoint.status === "running"
            ? { ...checkpoint, status: "failed" }
            : checkpoint,
        ),
      );
      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "The test purchase did not complete.",
      );
    } finally {
      setRunning(false);
    }
  }

  const passed = checkpoints.every(
    (checkpoint) => checkpoint.status === "passed",
  );
  return (
    <section
      aria-label="Run test purchase"
      className={styles.panel}
      id="test-purchase"
    >
      <header className={styles.header}>
        <div>
          <p>Seller launch proof</p>
          <h3>
            {passed ? "Launch test passed" : "Run a real commerce rehearsal"}
          </h3>
          <span>
            {testRoute.displayName} ·{" "}
            {formatAtomicPrice(testRoute.amount, testRoute.asset)}
          </span>
        </div>
        <span className={styles.mode}>
          {paymentMode === "mock"
            ? "Local mock payment"
            : paymentMode === "x402"
              ? "Base Sepolia testnet"
              : "Mode detected at runtime"}
        </span>
      </header>

      <ol className={styles.checkpoints} aria-live="polite">
        {checkpoints.map((checkpoint) => (
          <li aria-label={checkpoint.label} key={checkpoint.name}>
            <span className={styles.checkIcon} aria-hidden="true">
              {checkpoint.status === "passed" ? (
                <Check />
              ) : checkpoint.status === "running" ? (
                <LoaderCircle className={styles.spinner} />
              ) : checkpoint.status === "failed" ? (
                <CircleAlert />
              ) : (
                <Circle />
              )}
            </span>
            <div>
              <strong>{checkpoint.label}</strong>
              <small>{checkpoint.detail}</small>
            </div>
            <span className={styles.status}>
              {checkpointStatusLabel(checkpoint.status)}
            </span>
          </li>
        ))}
      </ol>

      {error ? (
        <p className={styles.error} role="alert">
          {error}
        </p>
      ) : null}

      <footer className={styles.actions}>
        <div>
          <ShieldCheck aria-hidden="true" />
          <span>
            {paymentMode === "mock"
              ? "Local mock mode uses no production funds."
              : paymentMode === "x402"
                ? "Base Sepolia testnet only; no mainnet funds."
                : "The signed challenge selects local mock or Base Sepolia testnet."}
          </span>
        </div>
        {transactionId ? (
          <Link
            className={buttonVariants()}
            href={`/dashboard/transactions/${transactionId}`}
          >
            <ReceiptText aria-hidden="true" /> Open verified transaction
          </Link>
        ) : (
          <Button
            disabled={running}
            onClick={() => void runTestPurchase()}
            type="button"
          >
            {running ? (
              <LoaderCircle className={styles.spinner} aria-hidden="true" />
            ) : error ? (
              <RotateCcw aria-hidden="true" />
            ) : (
              <ShieldCheck aria-hidden="true" />
            )}
            {running
              ? "Running test purchase"
              : error
                ? "Retry test purchase"
                : "Run test purchase"}
          </Button>
        )}
      </footer>
    </section>
  );
}

function withCheckpoint(
  checkpoints: Checkpoint[],
  name: CheckpointName,
  status: CheckpointStatus,
): Checkpoint[] {
  return checkpoints.map((checkpoint) =>
    checkpoint.name === name ? { ...checkpoint, status } : checkpoint,
  );
}

function checkpointStatusLabel(status: CheckpointStatus): string {
  switch (status) {
    case "running":
      return "Checking";
    case "passed":
      return "Passed";
    case "failed":
      return "Failed";
    default:
      return "Waiting";
  }
}

function requireWallet() {
  const wallet = getInjectedWallet();
  if (!wallet) {
    throw new Error(
      "Connect an EVM wallet with Base Sepolia USDC to run the testnet purchase.",
    );
  }
  return wallet;
}

async function createTestnetPaymentSignature(
  paymentRequired: string,
): Promise<string> {
  const wallet = requireWallet();
  const compatibility = await detectWalletCompatibility(wallet);
  if (!compatibility.compatible) {
    throw new Error(compatibility.message);
  }
  return createX402PaymentSignature(paymentRequired, wallet);
}

function assertTestPaymentTerms(
  result: CheckoutStartResult,
  challenge: DecodedPaymentRequired,
  route: TestPurchaseRoute,
  sellerSlug: string,
) {
  const intent = result.purchaseIntent;
  let resourcePath: string;
  try {
    resourcePath = new URL(challenge.resource.url).pathname;
  } catch {
    throw new Error("AgentPay returned an invalid payment resource.");
  }
  if (
    challenge.mode !== result.paymentMode ||
    intent.routeId !== route.routeId ||
    intent.productSlug !== route.productSlug ||
    intent.amount !== route.amount ||
    intent.asset !== route.asset ||
    intent.network !== route.network ||
    challenge.requirements.amount !== intent.amount ||
    challenge.requirements.network !== intent.network ||
    challenge.requirements.payTo.toLowerCase() !== intent.payTo.toLowerCase() ||
    resourcePath !== `/pay/${sellerSlug}${intent.requestPath}`
  ) {
    throw new Error(
      "AgentPay blocked the test because the payment terms did not match the published product.",
    );
  }
}
