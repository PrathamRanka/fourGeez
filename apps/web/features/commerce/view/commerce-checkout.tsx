"use client";

import {
  Bot,
  Check,
  CircleAlert,
  ExternalLink,
  LoaderCircle,
  LockKeyhole,
  ReceiptText,
  RotateCcw,
  ShieldCheck,
  WalletCards,
} from "lucide-react";
import Link from "next/link";
import { useId, useState } from "react";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  asPublishedInputSchema,
  buildSchemaDocument,
  schemaFieldLabel,
  schemaPropertyEntries,
  validatePublishedInput,
  type InputDrafts,
  type PublishedInputSchema,
} from "@/features/commerce/input-schema";
import type {
  CheckoutCompleteResult,
  CheckoutStartResult,
  CommerceChannel,
} from "@/features/commerce/model";
import { requestCheckout } from "@/features/commerce/checkout-client";
import {
  createX402PaymentSignature,
  decodePaymentRequired,
  detectWalletCompatibility,
  getInjectedWallet,
  PaymentCapabilityError,
  type PaymentRecoveryAction,
} from "@/features/commerce/x402-wallet";
import type { PublicProduct } from "@/features/storefront/model";
import {
  decimalToAtomicUnits,
  formatAtomicPrice,
  formatAtomicUnits,
} from "@/lib/money";
import styles from "./commerce-checkout.module.css";

type CheckoutStage =
  | "ready"
  | "starting"
  | "payment_ready"
  | "wallet_missing"
  | "signing"
  | "fulfilled"
  | "error";

type CheckoutError = {
  message: string;
  retryable: boolean;
  recoveryAction?: PaymentRecoveryAction;
};

export function CommerceCheckout({
  channel,
  product,
  sellerSlug,
  heading = channel === "agent" ? "Agent-selected checkout" : "Secure checkout",
  defaultRequestBody = channel === "agent" ? "{}" : "",
}: {
  channel: CommerceChannel;
  product: PublicProduct;
  sellerSlug: string;
  heading?: string;
  defaultRequestBody?: string;
}) {
  const [stage, setStage] = useState<CheckoutStage>("ready");
  const [confirmed, setConfirmed] = useState(false);
  const [maximumSpend, setMaximumSpend] = useState(
    formatAtomicUnits(product.amount),
  );
  const [requestBody, setRequestBody] = useState(defaultRequestBody);
  const [inputDrafts, setInputDrafts] = useState<InputDrafts>({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [startResult, setStartResult] = useState<CheckoutStartResult | null>(
    null,
  );
  const [completeResult, setCompleteResult] =
    useState<CheckoutCompleteResult | null>(null);
  const [checkoutError, setCheckoutError] = useState<CheckoutError | null>(
    null,
  );
  const [paymentSignature, setPaymentSignature] = useState<string | null>(null);
  const [buyerWalletAddress, setBuyerWalletAddress] = useState<string | null>(
    null,
  );

  const maximumSpendID = useId();
  const maximumSpendHelpID = useId();
  const requestBodyID = useId();
  const exactPrice = formatAtomicPrice(product.amount, product.asset);

  async function startCheckout() {
    setCheckoutError(null);
    setFieldErrors({});
    setStartResult(null);
    setCompleteResult(null);
    setPaymentSignature(null);
    setBuyerWalletAddress(null);
    let maximumAmount: string;
    let validatedRequestBody = requestBody;
    try {
      maximumAmount = decimalToAtomicUnits(maximumSpend);
      if (BigInt(maximumAmount) < BigInt(product.amount)) {
        throw new Error(
          `Maximum spend must cover the exact ${exactPrice} price.`,
        );
      }
    } catch (error) {
      setFieldErrors({
        maximumSpend:
          error instanceof Error
            ? error.message
            : "Enter a valid maximum spend.",
      });
      return;
    }
    try {
      if (channel === "browser") {
        const document = buildSchemaDocument(
          asPublishedInputSchema(product.inputSchema),
          inputDrafts,
        );
        if (!document.value || document.issues.length > 0) {
          setFieldErrors(
            Object.fromEntries(
              document.issues.map((issue) => [issue.field, issue.message]),
            ),
          );
          return;
        }
        validatedRequestBody = JSON.stringify(document.value);
      } else {
        const parsedRequest = requestBody.trim()
          ? (JSON.parse(requestBody) as unknown)
          : {};
        const issues = validatePublishedInput(
          asPublishedInputSchema(product.inputSchema),
          parsedRequest,
        );
        if (issues.length > 0) {
          setFieldErrors(
            Object.fromEntries(
              issues.map((issue) => [issue.field, issue.message]),
            ),
          );
          return;
        }
        validatedRequestBody = JSON.stringify(parsedRequest);
      }
    } catch (error) {
      setFieldErrors({
        requestBody:
          error instanceof Error
            ? error.message
            : "Check the purchase details.",
      });
      return;
    }

    setRequestBody(validatedRequestBody);
    setStage("starting");
    const response = await requestCheckout<CheckoutStartResult>(
      "/api/commerce/start",
      {
        channel,
        sellerSlug,
        productSlug: product.productSlug,
        routeId: product.routeId,
        maximumAmount,
        requestBody: validatedRequestBody,
      },
    );
    if (!response.ok) {
      setStage("error");
      setCheckoutError({
        message: response.error,
        retryable: response.retryable,
      });
      return;
    }
    try {
      assertPaymentTerms(response.value, product, sellerSlug);
    } catch (error) {
      setStage("error");
      setCheckoutError({
        message:
          error instanceof Error
            ? error.message
            : "AgentPay returned mismatched payment terms.",
        retryable: false,
      });
      return;
    }
    setStartResult(response.value);
    if (response.value.paymentMode === "x402") {
      const compatibility =
        await detectWalletCompatibility(getInjectedWallet());
      setBuyerWalletAddress(compatibility.address ?? null);
      if (!compatibility.compatible) {
        setCheckoutError({
          message: compatibility.message,
          retryable: true,
          recoveryAction: compatibility.recoveryAction,
        });
        setStage("wallet_missing");
        return;
      }
    }
    setStage("payment_ready");
  }

  async function completeCheckout() {
    if (!startResult) {
      return;
    }
    setCheckoutError(null);
    setStage("signing");
    try {
      const decoded = decodePaymentRequired(startResult.paymentRequired);
      const provider = getInjectedWallet();
      if (decoded.mode === "x402" && !provider) {
        setStage("wallet_missing");
        return;
      }
      const signedPayment =
        paymentSignature ??
        (decoded.mode === "mock"
          ? "mock-approved-proof"
          : await createX402PaymentSignature(
              startResult.paymentRequired,
              provider!,
            ));
      setPaymentSignature(signedPayment);
      const response = await requestCheckout<CheckoutCompleteResult>(
        "/api/commerce/complete",
        {
          channel,
          sellerSlug,
          requestBody,
          purchaseIntent: startResult.purchaseIntent,
          paymentSignature: signedPayment,
        },
      );
      if (!response.ok) {
        setStage("error");
        setCheckoutError({
          message: response.error,
          retryable: response.retryable,
          recoveryAction: response.recoveryAction,
        });
        if (response.recoveryAction === "sign_fresh_authorization") {
          setPaymentSignature(null);
        }
        return;
      }
      setCompleteResult(response.value);
      setStage("fulfilled");
    } catch (error) {
      setStage("error");
      setCheckoutError(walletCheckoutError(error));
    }
  }

  async function checkWalletAgain() {
    const compatibility = await detectWalletCompatibility(getInjectedWallet());
    setBuyerWalletAddress(compatibility.address ?? null);
    if (compatibility.compatible) {
      setCheckoutError(null);
      setStage("payment_ready");
      return;
    }
    setCheckoutError({
      message: compatibility.message,
      retryable: true,
      recoveryAction: compatibility.recoveryAction,
    });
    setStage("wallet_missing");
  }

  function resetCheckout() {
    setStartResult(null);
    setCompleteResult(null);
    setCheckoutError(null);
    setPaymentSignature(null);
    setBuyerWalletAddress(null);
    setStage("ready");
  }

  async function startFreshCheckout() {
    resetCheckout();
    await startCheckout();
  }

  return (
    <section className={styles.checkout} aria-labelledby="checkout-title">
      <header>
        <div className={styles.icon}>
          {channel === "agent" ? (
            <Bot aria-hidden="true" />
          ) : (
            <WalletCards aria-hidden="true" />
          )}
        </div>
        <div>
          <p>
            {channel === "agent" ? "Agent channel demo" : "Browser purchase"}
          </p>
          <h2 id="checkout-title">{heading}</h2>
          <small className={styles.guardLabel}>Protected x402 settlement</small>
          <small className={styles.testnetNotice}>
            Testnet only · Base Sepolia USDC · no real-money production
          </small>
        </div>
        <span>{exactPrice}</span>
      </header>

      {stage === "fulfilled" && completeResult ? (
        <div className={styles.success} aria-live="polite">
          <span>
            <Check aria-hidden="true" />
          </span>
          <p>Paid and fulfilled</p>
          <h3>The seller completed this purchase.</h3>
          <dl>
            <div>
              <dt>Transaction</dt>
              <dd>{completeResult.transactionId}</dd>
            </div>
            <div>
              <dt>Settlement</dt>
              <dd>
                {completeResult.settlementReference ?? "Verified by AgentPay"}
              </dd>
            </div>
          </dl>
          <div className={styles.fulfillment}>
            <ReceiptText aria-hidden="true" />
            <div>
              <strong>Fulfillment response</strong>
              <pre>{formatFulfillment(completeResult.fulfillment)}</pre>
            </div>
          </div>
          <div className={styles.actions}>
            {channel === "browser" ? (
              <>
                <Link
                  className={`${buttonVariants()} ${styles.action}`}
                  href={`/purchases/${encodeURIComponent(completeResult.transactionId)}#receipt-title`}
                >
                  <ReceiptText aria-hidden="true" /> View receipt
                </Link>
                <Link
                  className={`${buttonVariants({ variant: "outline" })} ${styles.action}`}
                  href={`/purchases/${encodeURIComponent(completeResult.transactionId)}#evidence-title`}
                >
                  <ShieldCheck aria-hidden="true" /> View evidence
                </Link>
                <form
                  action={`/purchases/${encodeURIComponent(completeResult.transactionId)}#buyer-dispute-title`}
                >
                  <Button
                    className={styles.action}
                    type="submit"
                    variant="outline"
                  >
                    <CircleAlert aria-hidden="true" /> Open dispute
                  </Button>
                </form>
              </>
            ) : null}
            <Button
              className={styles.action}
              type="button"
              variant="outline"
              onClick={resetCheckout}
            >
              Start another purchase
            </Button>
          </div>
        </div>
      ) : (
        <>
          <div className={styles.fields}>
            <label htmlFor={maximumSpendID}>
              <span>Maximum spend</span>
              <div className={styles.amountInput}>
                <input
                  aria-label="Maximum spend"
                  aria-describedby={
                    fieldErrors.maximumSpend
                      ? `${maximumSpendID}-error`
                      : maximumSpendHelpID
                  }
                  aria-invalid={fieldErrors.maximumSpend ? true : undefined}
                  disabled={stage !== "ready" && stage !== "error"}
                  id={maximumSpendID}
                  inputMode="decimal"
                  name="maximumSpend"
                  onChange={(event) => setMaximumSpend(event.target.value)}
                  value={maximumSpend}
                />
                <span>{product.asset}</span>
              </div>
              {fieldErrors.maximumSpend ? (
                <small
                  className={styles.fieldError}
                  id={`${maximumSpendID}-error`}
                  role="alert"
                >
                  {fieldErrors.maximumSpend}
                </small>
              ) : (
                <small id={maximumSpendHelpID}>
                  A ceiling only. Your wallet authorizes the seller&apos;s exact{" "}
                  {exactPrice} quote.
                </small>
              )}
            </label>
            {channel === "agent" ? (
              <label htmlFor={requestBodyID}>
                <span>Request body (JSON)</span>
                <textarea
                  aria-describedby={
                    fieldErrors.requestBody
                      ? `${requestBodyID}-error`
                      : undefined
                  }
                  aria-invalid={fieldErrors.requestBody ? true : undefined}
                  aria-label="Request body (JSON)"
                  disabled={stage !== "ready" && stage !== "error"}
                  id={requestBodyID}
                  name="requestBody"
                  onChange={(event) => setRequestBody(event.target.value)}
                  spellCheck={false}
                  value={requestBody}
                />
                {fieldErrors.requestBody ? (
                  <small
                    className={styles.fieldError}
                    id={`${requestBodyID}-error`}
                    role="alert"
                  >
                    {fieldErrors.requestBody}
                  </small>
                ) : (
                  <small>JSON must match the published product schema.</small>
                )}
              </label>
            ) : (
              <SchemaFields
                disabled={stage !== "ready" && stage !== "error"}
                drafts={inputDrafts}
                errors={fieldErrors}
                onChange={(path, value) =>
                  setInputDrafts((current) => ({
                    ...current,
                    [path]: value,
                  }))
                }
                schema={asPublishedInputSchema(product.inputSchema)}
              />
            )}
            <label className={styles.confirmation}>
              <input
                checked={confirmed}
                disabled={stage !== "ready" && stage !== "error"}
                onChange={(event) => setConfirmed(event.target.checked)}
                type="checkbox"
              />
              <span>
                I confirm the product, exact price, network and maximum spend.
              </span>
            </label>
          </div>

          <CommerceStatus
            channel={channel}
            error={checkoutError}
            exactPrice={exactPrice}
            paymentMode={startResult?.paymentMode}
            stage={stage}
          />

          {startResult &&
          (stage === "payment_ready" ||
            stage === "wallet_missing" ||
            stage === "signing" ||
            stage === "error") ? (
            <dl className={styles.paymentParties} aria-label="Payment parties">
              {buyerWalletAddress ? (
                <div>
                  <dt>Buyer wallet</dt>
                  <dd>{buyerWalletAddress}</dd>
                </div>
              ) : null}
              <div>
                <dt>Seller wallet</dt>
                <dd>{startResult.purchaseIntent.payTo}</dd>
              </div>
              <div>
                <dt>
                  {startResult.transactionId
                    ? "Transaction trace"
                    : "Request trace"}
                </dt>
                <dd>{startResult.traceId ?? startResult.transactionId}</dd>
              </div>
            </dl>
          ) : null}

          <div className={styles.actions}>
            {stage === "ready" ? (
              <Button
                className={styles.action}
                type="button"
                disabled={!confirmed}
                onClick={startCheckout}
              >
                <LockKeyhole aria-hidden="true" /> Review exact payment
              </Button>
            ) : null}
            {stage === "starting" ? (
              <Button className={styles.action} type="button" disabled>
                <LoaderCircle className="animate-spin" aria-hidden="true" />
                Freezing quote
              </Button>
            ) : null}
            {stage === "payment_ready" ? (
              <Button
                className={styles.action}
                type="button"
                onClick={completeCheckout}
              >
                <WalletCards aria-hidden="true" />
                {startResult?.paymentMode === "mock"
                  ? "Complete local demo payment"
                  : `Confirm ${exactPrice} in wallet`}
              </Button>
            ) : null}
            {stage === "signing" ? (
              <Button className={styles.action} type="button" disabled>
                <LoaderCircle className="animate-spin" aria-hidden="true" />
                Verifying and fulfilling
              </Button>
            ) : null}
            {stage === "wallet_missing" ? (
              <Button
                className={styles.action}
                type="button"
                variant="outline"
                onClick={() => void checkWalletAgain()}
              >
                <RotateCcw aria-hidden="true" /> Check for wallet again
              </Button>
            ) : null}
            {stage === "error" && checkoutError?.retryable ? (
              <Button
                className={styles.action}
                type="button"
                variant="outline"
                onClick={
                  checkoutError.recoveryAction === "retry_same_payment" ||
                  checkoutError.recoveryAction === "retry_same_request" ||
                  checkoutError.recoveryAction === "sign_fresh_authorization" ||
                  checkoutError.recoveryAction === "connect_wallet" ||
                  checkoutError.recoveryAction === "switch_network"
                    ? completeCheckout
                    : checkoutError.recoveryAction === "start_new_checkout"
                      ? startFreshCheckout
                      : resetCheckout
                }
              >
                <RotateCcw aria-hidden="true" />
                {paymentRecoveryLabel(checkoutError.recoveryAction)}
              </Button>
            ) : null}
          </div>

          <footer className={styles.footer}>
            <ShieldCheck aria-hidden="true" />
            <span>
              Wallet confirmation is your consent. AgentPay rechecks the seller,
              quote and destination before settlement, then fulfills once.
            </span>
          </footer>
        </>
      )}
    </section>
  );
}

function CommerceStatus({
  channel,
  error,
  exactPrice,
  paymentMode,
  stage,
}: {
  channel: CommerceChannel;
  error: CheckoutError | null;
  exactPrice: string;
  paymentMode?: "mock" | "x402";
  stage: CheckoutStage;
}) {
  if (stage === "wallet_missing") {
    return (
      <div className={`${styles.status} ${styles.warning}`} role="status">
        <CircleAlert aria-hidden="true" />
        <div>
          <strong>Wallet required</strong>
          <span>
            {error?.message ??
              "Install or open an EVM wallet that supports Base Sepolia, then return here. The purchase has not been charged."}
          </span>
          <a href="/docs" target="_blank" rel="noreferrer">
            Open wallet guidance <ExternalLink aria-hidden="true" />
          </a>
        </div>
      </div>
    );
  }
  if (stage === "error" && error) {
    return (
      <div className={`${styles.status} ${styles.error}`} role="alert">
        <CircleAlert aria-hidden="true" />
        <div>
          <strong>Checkout did not complete</strong>
          <span>{error.message}</span>
          {error.recoveryAction === "retry_same_payment" ? (
            <span>
              AgentPay will retry the same signed payment and will not request a
              second wallet authorization.
            </span>
          ) : null}
        </div>
      </div>
    );
  }
  if (stage === "payment_ready") {
    return (
      <div className={`${styles.status} ${styles.ready}`} role="status">
        <ShieldCheck aria-hidden="true" />
        <div>
          <strong>Payment ready</strong>
          <span>
            {paymentMode === "mock"
              ? `Local demo mode will simulate the exact ${exactPrice} settlement.`
              : `Your wallet will sign one exact ${exactPrice} Base Sepolia authorization.`}
          </span>
        </div>
      </div>
    );
  }
  return (
    <div className={styles.status} role="status" aria-live="polite">
      <LockKeyhole aria-hidden="true" />
      <div>
        <strong>
          {stage === "starting" ? "Freezing quote" : "No charge yet"}
        </strong>
        <span>
          {stage === "starting"
            ? "AgentPay is creating an immutable intent and requesting x402 terms."
            : channel === "agent"
              ? "The agent may select an offer, but only your wallet can authorize payment."
              : "Review the request and confirm your spending ceiling to continue."}
        </span>
      </div>
    </div>
  );
}

function walletCheckoutError(error: unknown): CheckoutError {
  if (error instanceof PaymentCapabilityError) {
    return {
      message: error.message,
      retryable: true,
      recoveryAction: error.recoveryAction,
    };
  }
  return {
    message:
      error instanceof Error
        ? error.message
        : "The wallet could not authorize this payment.",
    retryable: true,
    recoveryAction: "sign_fresh_authorization",
  };
}

function paymentRecoveryLabel(action?: PaymentRecoveryAction): string {
  switch (action) {
    case "connect_wallet":
      return "Connect wallet and retry";
    case "switch_network":
      return "Retry network switch";
    case "sign_fresh_authorization":
      return "Retry wallet authorization";
    case "retry_same_request":
      return "Retry payment verification";
    case "retry_same_payment":
      return "Retry settlement check";
    case "start_new_checkout":
      return "Start new checkout";
    default:
      return "Try again";
  }
}

function assertPaymentTerms(
  result: CheckoutStartResult,
  product: PublicProduct,
  sellerSlug: string,
) {
  const intent = result.purchaseIntent;
  const challenge = decodePaymentRequired(result.paymentRequired);
  const expectedPaidPath = `/pay/${sellerSlug}${intent.requestPath}`;
  let resourcePath: string;
  try {
    resourcePath = new URL(challenge.resource.url).pathname;
  } catch {
    throw new Error("AgentPay returned an invalid payment resource.");
  }
  if (
    intent.routeId !== product.routeId ||
    intent.productSlug !== product.productSlug ||
    intent.amount !== product.amount ||
    intent.network !== product.network ||
    challenge.requirements.amount !== intent.amount ||
    challenge.requirements.network !== intent.network ||
    challenge.requirements.payTo.toLowerCase() !== intent.payTo.toLowerCase() ||
    resourcePath !== expectedPaidPath
  ) {
    throw new Error(
      "AgentPay blocked checkout because the signed payment terms did not match the displayed product.",
    );
  }
}

function SchemaFields({
  disabled,
  drafts,
  errors,
  onChange,
  schema,
  path = "",
}: {
  disabled: boolean;
  drafts: InputDrafts;
  errors: Record<string, string>;
  onChange: (path: string, value: string | boolean) => void;
  schema: PublishedInputSchema;
  path?: string;
}) {
  const entries = schemaPropertyEntries(schema);
  if (entries.length === 0) {
    return (
      <div className={styles.noInput}>
        <strong>No additional details required</strong>
        <small>This product accepts an empty request.</small>
      </div>
    );
  }
  return (
    <fieldset className={styles.schemaFields}>
      <legend>Purchase details</legend>
      {entries.map(([name, fieldSchema]) => {
        const fieldPath = path ? `${path}.${name}` : name;
        if (fieldSchema.type === "object") {
          return (
            <fieldset className={styles.nestedFields} key={fieldPath}>
              <legend>{schemaFieldLabel(name)}</legend>
              {fieldSchema.description ? (
                <small>{fieldSchema.description}</small>
              ) : null}
              <SchemaFields
                disabled={disabled}
                drafts={drafts}
                errors={errors}
                onChange={onChange}
                path={fieldPath}
                schema={fieldSchema}
              />
            </fieldset>
          );
        }
        const required = schema.required?.includes(name) ?? false;
        const label = `${schemaFieldLabel(name)}${required ? " (required)" : ""}`;
        const idBase = fieldPath.replaceAll(".", "-");
        const errorID = `${idBase}-error`;
        const descriptionID = `${idBase}-description`;
        const describedBy = errors[fieldPath]
          ? errorID
          : fieldSchema.description
            ? descriptionID
            : undefined;
        return (
          <label key={fieldPath}>
            <span>{label}</span>
            <SchemaControl
              describedBy={describedBy}
              disabled={disabled}
              fieldPath={fieldPath}
              fieldSchema={fieldSchema}
              invalid={Boolean(errors[fieldPath])}
              label={label}
              onChange={onChange}
              required={required}
              value={drafts[fieldPath]}
            />
            {errors[fieldPath] ? (
              <small className={styles.fieldError} id={errorID} role="alert">
                {errors[fieldPath]}
              </small>
            ) : fieldSchema.description ? (
              <small id={descriptionID}>{fieldSchema.description}</small>
            ) : null}
          </label>
        );
      })}
    </fieldset>
  );
}

function SchemaControl({
  describedBy,
  disabled,
  fieldPath,
  fieldSchema,
  invalid,
  label,
  onChange,
  required,
  value,
}: {
  describedBy?: string;
  disabled: boolean;
  fieldPath: string;
  fieldSchema: PublishedInputSchema;
  invalid: boolean;
  label: string;
  onChange: (path: string, value: string | boolean) => void;
  required: boolean;
  value: string | boolean | undefined;
}) {
  if (fieldSchema.type === "boolean") {
    return (
      <input
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        aria-label={label}
        checked={value === true}
        disabled={disabled}
        onChange={(event) => onChange(fieldPath, event.target.checked)}
        required={required}
        type="checkbox"
      />
    );
  }
  if (fieldSchema.enum?.every((choice) => typeof choice === "string")) {
    return (
      <select
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        aria-label={label}
        disabled={disabled}
        onChange={(event) => onChange(fieldPath, event.target.value)}
        required={required}
        value={typeof value === "string" ? value : ""}
      >
        <option value="">Select an option</option>
        {(fieldSchema.enum as string[]).map((choice) => (
          <option key={choice} value={choice}>
            {choice}
          </option>
        ))}
      </select>
    );
  }
  if (
    fieldSchema.type === "array" ||
    (fieldSchema.type === "string" && (fieldSchema.maxLength ?? 0) > 160)
  ) {
    return (
      <textarea
        aria-describedby={describedBy}
        aria-invalid={invalid || undefined}
        aria-label={label}
        disabled={disabled}
        maxLength={fieldSchema.maxLength}
        onChange={(event) => onChange(fieldPath, event.target.value)}
        placeholder={
          fieldSchema.type === "array" ? '["first item"]' : undefined
        }
        required={required}
        value={typeof value === "string" ? value : ""}
      />
    );
  }
  return (
    <input
      aria-describedby={describedBy}
      aria-invalid={invalid || undefined}
      aria-label={label}
      disabled={disabled}
      max={fieldSchema.maximum}
      maxLength={fieldSchema.maxLength}
      min={fieldSchema.minimum}
      minLength={fieldSchema.minLength}
      onChange={(event) => onChange(fieldPath, event.target.value)}
      pattern={fieldSchema.pattern}
      required={required}
      type={
        fieldSchema.type === "number" || fieldSchema.type === "integer"
          ? "number"
          : fieldSchema.format === "email"
            ? "email"
            : "text"
      }
      value={typeof value === "string" ? value : ""}
    />
  );
}

function formatFulfillment(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value, null, 2);
}
