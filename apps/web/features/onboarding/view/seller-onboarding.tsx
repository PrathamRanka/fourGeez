"use client";

import {
  Check,
  CheckCircle2,
  Circle,
  Clipboard,
  Code2,
  KeyRound,
  LoaderCircle,
  ShieldCheck,
  Store,
  WalletCards,
} from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { OperationState } from "@/components/dashboard/operation-state";
import { Button, buttonVariants } from "@/components/ui/button";
import type {
  CredentialCreated,
  IntegrationCredential,
  MCPHost,
  OnboardingActions,
  OnboardingSnapshot,
  OnboardingStepName,
  Seller,
  SellerOnboardingState,
} from "@/features/onboarding/model";
import {
  createMCPConfiguration,
  createPowerShellSetup,
  setupPrompt,
} from "@/features/onboarding/model";
import { SellerTestPurchase } from "@/features/onboarding/view/seller-test-purchase";
import styles from "./seller-onboarding.module.css";

type SellerOnboardingProps = {
  actions: OnboardingActions;
  apiOrigin?: string;
  initialSnapshot: OnboardingSnapshot;
};

type EthereumProvider = {
  request: (request: {
    method: string;
    params?: unknown[];
  }) => Promise<unknown>;
};

declare global {
  interface Window {
    ethereum?: EthereumProvider;
  }
}

const supportedAsset = "USDC";
const supportedNetwork = "eip155:84532";
const eligibilitySteps = [
  "account_verified",
  "storefront_created",
  "service_connection_verified",
  "subscription_active",
  "payment_destination_verified",
] as const satisfies readonly OnboardingStepName[];
const testPurchasePrerequisiteSteps = [
  ...eligibilitySteps,
  "project_key_created",
  "connector_verified",
  "product_configured",
] as const satisfies readonly OnboardingStepName[];

const prerequisiteDetails: Record<
  (typeof eligibilitySteps)[number],
  { label: string; href: string; action: string }
> = {
  account_verified: {
    label: "Verified seller account and required profile",
    href: "/verify",
    action: "Complete account profile",
  },
  storefront_created: {
    label: "Seller storefront created",
    href: "/dashboard/onboarding#storefront",
    action: "Complete storefront",
  },
  service_connection_verified: {
    label: "Active HTTPS service endpoint and signing readiness",
    href: "/dashboard/onboarding#service-readiness",
    action: "Complete service readiness",
  },
  subscription_active: {
    label: "Active testnet launch entitlement",
    href: "/dashboard/onboarding#launch-entitlement",
    action: "Request launch entitlement",
  },
  payment_destination_verified: {
    label: "Seller-verified USDC payout on Base Sepolia",
    href: "/dashboard/onboarding#payment-destination",
    action: "Verify payout destination",
  },
};

export function SellerOnboarding({
  actions,
  apiOrigin = "http://localhost:8080",
  initialSnapshot,
}: SellerOnboardingProps) {
  const [seller, setSeller] = useState<Seller | null>(initialSnapshot.seller);
  const [paymentDestinations, setPaymentDestinations] = useState(
    initialSnapshot.paymentDestinations,
  );
  const [credentials, setCredentials] = useState(initialSnapshot.credentials);
  const [onboarding, setOnboarding] = useState(initialSnapshot.onboarding);
  const [createdCredential, setCreatedCredential] =
    useState<CredentialCreated | null>(null);
  const [host, setHost] = useState<MCPHost>("claude-code");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [pendingStep, setPendingStep] = useState<string | null>(null);

  const stepComplete = (name: OnboardingStepName) =>
    onboarding.steps.some(
      (step) => step.name === name && step.status === "complete",
    );
  const eligible = eligibilitySteps.every(stepComplete);
  const testPurchaseEligible =
    testPurchasePrerequisiteSteps.every(stepComplete);
  const testableRoute =
    initialSnapshot.testableRoutes.find(
      (route) => route.lifecycleStatus === "published" && route.enabled,
    ) ?? null;
  const sellerSlug = seller?.slug ?? "";
  const latestCredential = credentials.at(-1) ?? null;
  const activeCredential =
    credentials.find(
      (credential) => credentialLifecycle(credential) === "active",
    ) ?? null;
  const projectCredential = createdCredential ?? activeCredential;
  const lifecycle = createdCredential
    ? "active"
    : credentialLifecycle(latestCredential);
  const connectorState =
    lifecycle === "revoked" || lifecycle === "expired"
      ? lifecycle
      : stepComplete("connector_verified")
        ? "connected"
        : "disconnected";
  const completedSteps = onboarding.steps.filter(
    (step) => step.status === "complete",
  ).length;
  const totalSteps = onboarding.steps.length || 10;
  const mcpConfiguration = useMemo(() => createMCPConfiguration(host), [host]);
  const powerShellSetup = useMemo(
    () => createPowerShellSetup(apiOrigin, host),
    [apiOrigin, host],
  );

  if (seller?.status === "suspended") {
    return <OperationState kind="seller_suspended" />;
  }

  async function submitStorefront(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setErrorMessage(null);
    setPendingStep("storefront");
    const formData = new FormData(event.currentTarget);
    const result = await actions.createStorefront({
      name: String(formData.get("name") ?? "").trim(),
      slug: String(formData.get("slug") ?? "").trim(),
      upstreamBaseUrl: String(formData.get("upstreamBaseUrl") ?? "").trim(),
    });
    setPendingStep(null);
    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }
    setSeller(result.value);
    setOnboarding((current) => markStepComplete(current, "storefront_created"));
    window.history.replaceState(null, "", "/dashboard/onboarding");
  }

  async function activateService() {
    if (!seller || seller.status !== "draft") return;
    setErrorMessage(null);
    setPendingStep("service");
    const result = await actions.activateSellerService({
      sellerId: seller.sellerId,
      expectedVersion: seller.version,
    });
    setPendingStep(null);
    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }
    setSeller(result.value);
    setOnboarding((current) =>
      markStepComplete(current, "service_connection_verified"),
    );
  }

  async function verifyPayoutAddress(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!seller) return;
    if (!window.ethereum) {
      setErrorMessage(
        "Install or open an EVM-compatible browser wallet to continue.",
      );
      return;
    }
    const formData = new FormData(event.currentTarget);
    const address = String(formData.get("address") ?? "").trim();
    setErrorMessage(null);
    setPendingStep("wallet");
    try {
      const accountResult = await window.ethereum.request({
        method: "eth_requestAccounts",
      });
      const connectedAddress = Array.isArray(accountResult)
        ? String(accountResult[0] ?? "")
        : "";
      if (
        !connectedAddress ||
        connectedAddress.toLowerCase() !== address.toLowerCase()
      ) {
        throw new Error(
          "The connected wallet must match the payout address you entered.",
        );
      }
      const preparedResult = await actions.preparePaymentDestination({
        sellerId: seller.sellerId,
        asset: supportedAsset,
        network: supportedNetwork,
        address,
      });
      if (!preparedResult.ok) {
        setErrorMessage(preparedResult.error);
        return;
      }
      const signature = await window.ethereum.request({
        method: "personal_sign",
        params: [preparedResult.value.challenge, address],
      });
      if (typeof signature !== "string" || !signature) {
        throw new Error("The wallet did not return a signature.");
      }
      const verificationResult = await actions.verifyPaymentDestination({
        sellerId: seller.sellerId,
        destinationId: preparedResult.value.destination.destinationId,
        challenge: preparedResult.value.challenge,
        signature,
      });
      if (!verificationResult.ok) {
        setErrorMessage(verificationResult.error);
        return;
      }
      setPaymentDestinations((current) => [
        ...current.filter(
          (destination) =>
            destination.destinationId !==
            verificationResult.value.destinationId,
        ),
        verificationResult.value,
      ]);
      setOnboarding((current) =>
        markStepComplete(current, "payment_destination_verified"),
      );
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? error.message
          : "Wallet verification did not complete.",
      );
    } finally {
      setPendingStep(null);
    }
  }

  async function issueCredential() {
    if (!seller || !eligible) return;
    setErrorMessage(null);
    setPendingStep("credential");
    const result = await actions.createIntegrationCredential({
      sellerId: seller.sellerId,
    });
    setPendingStep(null);
    if (!result.ok) {
      setErrorMessage(result.error);
      return;
    }
    setCreatedCredential(result.value);
    setCredentials((current) => [...current, result.value]);
    setOnboarding((current) =>
      markStepComplete(current, "project_key_created"),
    );
  }

  async function copyText(value: string) {
    await navigator.clipboard.writeText(value);
  }

  return (
    <div
      className={styles.workspace}
      role="region"
      aria-label="Storefront launch sequence"
    >
      <aside className="onboarding-summary" aria-label="Launch progress">
        <p className="dashboard-eyebrow">Launch Rail</p>
        <h1>Launch your storefront</h1>
        <p>
          Complete the authoritative seller prerequisites before connecting a
          coding agent. Commercial changes always stay behind your approval.
        </p>
        <div className="onboarding-progress" aria-label="Onboarding progress">
          <div>
            <span>
              {completedSteps} of {totalSteps} complete
            </span>
            <span>{Math.round((completedSteps / totalSteps) * 100)}%</span>
          </div>
          <div className="onboarding-progress-track" aria-hidden="true">
            <span
              style={{ transform: `scaleX(${completedSteps / totalSteps})` }}
            />
          </div>
        </div>
        <PrerequisiteChecklist onboarding={onboarding} />
      </aside>

      <div className="onboarding-steps">
        {errorMessage ? (
          <div className="dashboard-error" role="alert">
            {errorMessage}
          </div>
        ) : null}

        <div className="onboarding-flow" aria-label="Launch tasks">
          {!seller ? (
            <OnboardingStep
              id="storefront"
              number="01"
              icon={Store}
              title="Create your seller profile"
              description="Name the storefront and configure the HTTPS service origin that will fulfill purchases."
              complete={false}
              current
              locked={false}
            >
              <form
                aria-label="Create storefront"
                className="onboarding-form"
                onSubmit={submitStorefront}
              >
                <label>
                  <span>Storefront name</span>
                  <input name="name" required maxLength={120} />
                </label>
                <label>
                  <span>Storefront URL name</span>
                  <input
                    name="slug"
                    required
                    minLength={3}
                    maxLength={48}
                    pattern="[a-z0-9-]+"
                  />
                </label>
                <label className="onboarding-field-wide">
                  <span>Service API URL</span>
                  <input
                    name="upstreamBaseUrl"
                    required
                    type="url"
                    pattern="https://.*"
                    placeholder="https://api.example.com"
                  />
                </label>
                <Button type="submit" disabled={pendingStep === "storefront"}>
                  {pendingStep === "storefront" ? (
                    <LoaderCircle className="animate-spin" aria-hidden="true" />
                  ) : null}
                  Create storefront
                </Button>
              </form>
            </OnboardingStep>
          ) : null}

          {seller ? (
            <OnboardingStep
              id="service-readiness"
              number="01"
              icon={Store}
              title="Seller profile and HTTPS service"
              description="The service origin must be HTTPS and enabled for AgentPay public-key execution-capability verification."
              complete={
                stepComplete("account_verified") &&
                stepComplete("storefront_created") &&
                stepComplete("service_connection_verified")
              }
              current={!stepComplete("service_connection_verified")}
              detail={seller.upstreamBaseUrl}
              locked={false}
            >
              <div className="onboarding-action-row">
                <div>
                  <strong>{seller.name}</strong>
                  <span>{seller.upstreamBaseUrl}</span>
                </div>
                <div className="onboarding-action-row">
                  {seller.status === "draft" ? (
                    <Button
                      type="button"
                      disabled={pendingStep === "service"}
                      onClick={activateService}
                    >
                      {pendingStep === "service" ? (
                        <LoaderCircle
                          className="animate-spin"
                          aria-hidden="true"
                        />
                      ) : null}
                      Enable secure service
                    </Button>
                  ) : (
                    <span className="status-ready">
                      ES256 verification enabled
                    </span>
                  )}
                  <Link
                    className={buttonVariants({ variant: "outline" })}
                    href="/dashboard/settings"
                  >
                    Review service settings
                  </Link>
                </div>
              </div>
            </OnboardingStep>
          ) : null}

          {seller ? (
            <OnboardingStep
              id="launch-entitlement"
              number="02"
              icon={ShieldCheck}
              title="Testnet launch entitlement"
              description="Stripe checkout is disabled for this launch. AgentPay must provision an active testnet entitlement."
              complete={stepComplete("subscription_active")}
              current={!stepComplete("subscription_active")}
              locked={false}
            >
              <div className="sandbox-readiness">
                <span className="status-waiting">
                  {stepComplete("subscription_active")
                    ? "Entitlement active"
                    : "Provisioning required"}
                </span>
                <p>
                  Contact the AgentPay launch operator if this prerequisite is
                  still blocked.
                </p>
              </div>
            </OnboardingStep>
          ) : null}

          {seller ? (
            <OnboardingStep
              id="payment-destination"
              number="03"
              icon={WalletCards}
              title="Verify your payout destination"
              description="Enter your own address, select a supported testnet pair, then prove that the connected wallet controls it."
              complete={stepComplete("payment_destination_verified")}
              current={!stepComplete("payment_destination_verified")}
              detail={
                paymentDestinations.find((item) => item.status === "active")
                  ?.address
              }
              locked={false}
            >
              <form
                className="onboarding-form"
                aria-label="Verify payout destination"
                onSubmit={verifyPayoutAddress}
              >
                <p className="onboarding-muted-state" role="note">
                  Testnet only · Base Sepolia USDC · no real-money production
                </p>
                <label>
                  <span>Asset and network</span>
                  <select name="assetNetwork" defaultValue="USDC:eip155:84532">
                    <option value="USDC:eip155:84532">
                      USDC · Base Sepolia
                    </option>
                  </select>
                </label>
                <label>
                  <span>Payout address</span>
                  <input
                    name="address"
                    required
                    pattern="0x[a-fA-F0-9]{40}"
                    placeholder="0x…"
                  />
                </label>
                <Button type="submit" disabled={pendingStep === "wallet"}>
                  {pendingStep === "wallet" ? (
                    <LoaderCircle className="animate-spin" aria-hidden="true" />
                  ) : null}
                  Verify payout address
                </Button>
              </form>
            </OnboardingStep>
          ) : null}

          {!eligible ? (
            <section className="onboarding-step" data-current="true">
              <header>
                <span className="onboarding-step-number">04</span>
                <span className="onboarding-step-icon">
                  <KeyRound className="size-4" aria-hidden="true" />
                </span>
                <div>
                  <h2>Coding-agent connection remains locked</h2>
                  <p>
                    Complete every prerequisite above. Project-key creation and
                    connector setup are intentionally hidden until the server
                    marks this seller eligible.
                  </p>
                </div>
                <span className="onboarding-step-status">Blocked</span>
              </header>
            </section>
          ) : (
            <>
              <OnboardingStep
                id="project-credential"
                number="04"
                icon={KeyRound}
                title="Project credential"
                description="Create a scoped bootstrap key. The raw value is never recoverable after this page leaves memory."
                complete={lifecycle === "active"}
                current={!projectCredential}
                detail={projectCredential?.label}
                locked={false}
              >
                <div className="onboarding-copy-grid">
                  {createdCredential ? (
                    <div className="credential-secret">
                      <strong>
                        Save this project connection key now. It is shown only
                        once.
                      </strong>
                      <code>{createdCredential.token}</code>
                    </div>
                  ) : activeCredential ? (
                    <div className="credential-secret">
                      <strong>Project key active — secret hidden</strong>
                      <p className="onboarding-muted-state">
                        AgentPay stores only a digest. Use the key you saved
                        when it was created.
                      </p>
                    </div>
                  ) : (
                    <Button
                      type="button"
                      disabled={pendingStep === "credential"}
                      onClick={issueCredential}
                    >
                      {pendingStep === "credential" ? (
                        <LoaderCircle
                          className="animate-spin"
                          aria-hidden="true"
                        />
                      ) : null}
                      Create project connection key
                    </Button>
                  )}
                </div>
              </OnboardingStep>

              {projectCredential ||
              lifecycle === "revoked" ||
              lifecycle === "expired" ? (
                <OnboardingStep
                  id="connector"
                  number="05"
                  icon={Code2}
                  title="Connect and diagnose MCP"
                  description="The local connector exchanges the project key for short-lived cloud access. The project key is never sent to /mcp."
                  complete={connectorState === "connected"}
                  current
                  detail={`Connector ${connectorState}`}
                  locked={false}
                >
                  <div className="onboarding-copy-grid">
                    <div className="sandbox-readiness">
                      <span
                        className={
                          connectorState === "connected"
                            ? "status-ready"
                            : "status-waiting"
                        }
                      >
                        Connector {connectorState}
                      </span>
                      <p>{connectorGuidance(connectorState)}</p>
                    </div>

                    {lifecycle === "active" ? (
                      <>
                        <label>
                          <span>Configuration host</span>
                          <select
                            aria-label="Configuration host"
                            value={host}
                            onChange={(event) =>
                              setHost(event.target.value as MCPHost)
                            }
                          >
                            <option value="claude-code">Claude Code</option>
                            <option value="codex">Codex</option>
                            <option value="generic-mcp">Generic MCP</option>
                          </select>
                        </label>
                        <CopyBlock
                          label="Host configuration"
                          value={mcpConfiguration}
                          onCopy={copyText}
                        />
                        <CopyBlock
                          label="Windows PowerShell setup"
                          value={powerShellSetup}
                          onCopy={copyText}
                        />
                        <CopyBlock
                          label="Seller-safe setup prompt"
                          value={setupPrompt}
                          onCopy={copyText}
                        />
                      </>
                    ) : (
                      <div className="credential-secret">
                        <strong>
                          {lifecycle === "revoked"
                            ? "Project key revoked"
                            : "Project key expired"}
                        </strong>
                        <p className="onboarding-muted-state">
                          Return to the project credential step and create a new
                          project key before retrying the connector.
                        </p>
                      </div>
                    )}
                  </div>
                </OnboardingStep>
              ) : null}

              <OnboardingStep
                id="validation"
                number="06"
                icon={ShieldCheck}
                title="Connector validation"
                description="Connector authorization is recorded on the first authenticated MCP request. The commerce rehearsal appears after a published product is available."
                complete={connectorState === "connected"}
                current={connectorState !== "connected"}
                locked={connectorState !== "connected"}
              >
                <div className="sandbox-readiness">
                  <span
                    className={
                      stepComplete("sandbox_purchase")
                        ? "status-ready"
                        : "status-waiting"
                    }
                  >
                    {stepComplete("sandbox_purchase")
                      ? "Connector and prior sandbox verified"
                      : connectorState === "connected"
                        ? "Connector validation passed"
                        : "Validation not run"}
                  </span>
                  <p>
                    {connectorState === "connected"
                      ? "Authenticated MCP initialization passed. Generate the verification endpoint and tests, then run sandbox_validate_route. Retry the PowerShell preflight after correcting any reported error."
                      : "Run the copyable PowerShell preflight. If it fails, confirm the API URL, key state, launch entitlement, and network access before retrying."}
                  </p>
                </div>
              </OnboardingStep>

              <SellerTestPurchase
                eligible={testPurchaseEligible}
                route={testableRoute}
                sellerSlug={sellerSlug}
                verifyPurchase={actions.verifySellerTestPurchase}
                onVerified={(verification) =>
                  setOnboarding(verification.onboarding)
                }
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function PrerequisiteChecklist({
  onboarding,
}: {
  onboarding: SellerOnboardingState;
}) {
  return (
    <ul className="onboarding-checklist" aria-label="MCP prerequisites">
      {eligibilitySteps.map((name) => {
        const complete = onboarding.steps.some(
          (step) => step.name === name && step.status === "complete",
        );
        const detail = prerequisiteDetails[name];
        return (
          <li key={name} data-complete={complete}>
            {complete ? (
              <Check aria-hidden="true" />
            ) : (
              <Circle aria-hidden="true" />
            )}
            <span>
              {detail.label}
              {!complete ? (
                <>
                  {" "}
                  <Link href={detail.href}>{detail.action}</Link>
                </>
              ) : null}
            </span>
          </li>
        );
      })}
    </ul>
  );
}

function CopyBlock({
  label,
  value,
  onCopy,
}: {
  label: string;
  value: string;
  onCopy: (value: string) => Promise<void>;
}) {
  return (
    <div className="onboarding-code-block">
      <div>
        <span>{label}</span>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label={`Copy ${label}`}
          onClick={() => onCopy(value)}
        >
          <Clipboard aria-hidden="true" />
          Copy
        </Button>
      </div>
      <pre aria-label={label}>{value}</pre>
    </div>
  );
}

function credentialLifecycle(
  credential: IntegrationCredential | null,
): "active" | "expired" | "revoked" | "none" {
  if (!credential) return "none";
  if (credential.revokedAt) return "revoked";
  if (credential.expiresAt && Date.parse(credential.expiresAt) <= Date.now()) {
    return "expired";
  }
  return "active";
}

function connectorGuidance(
  state: "connected" | "disconnected" | "expired" | "revoked",
): string {
  if (state === "connected") {
    return "Authenticated MCP initialization passed. AgentPay recorded this connector without treating key creation as connection proof.";
  }
  if (state === "revoked") {
    return "This key was revoked. Create a new project key, update the PowerShell session, and rerun the connector preflight.";
  }
  if (state === "expired") {
    return "This key expired. Create a new project key and rerun the connector preflight.";
  }
  return "No authenticated connector has reached AgentPay yet. Save the host config, run the Windows preflight, then refresh this page.";
}

function markStepComplete(
  onboarding: SellerOnboardingState,
  name: OnboardingStepName,
): SellerOnboardingState {
  return {
    ...onboarding,
    steps: onboarding.steps.map((step) =>
      step.name === name
        ? { ...step, status: "complete", blocking: false }
        : step,
    ),
  };
}

type OnboardingStepProps = {
  children: React.ReactNode;
  complete: boolean;
  current: boolean;
  description: string;
  detail?: string;
  icon: React.ComponentType<{
    className?: string;
    "aria-hidden"?: boolean | "true" | "false";
  }>;
  id?: string;
  locked: boolean;
  number: string;
  title: string;
};

function OnboardingStep({
  children,
  complete,
  current,
  description,
  detail,
  icon: Icon,
  id,
  locked,
  number,
  title,
}: OnboardingStepProps) {
  return (
    <section
      id={id}
      className="onboarding-step"
      data-complete={complete}
      data-current={current}
      data-locked={locked}
      aria-label={`${number} ${title}`}
      aria-current={current ? "step" : undefined}
    >
      <header>
        <span className="onboarding-step-number">{number}</span>
        <span className="onboarding-step-icon">
          <Icon className="size-4" aria-hidden="true" />
        </span>
        <div>
          <h2>{title}</h2>
          <p>{description}</p>
          {detail ? (
            <span className="onboarding-step-detail">{detail}</span>
          ) : null}
        </div>
        <span className="onboarding-step-status">
          {current
            ? "Current"
            : complete
              ? "Complete"
              : locked
                ? "Locked"
                : "Pending"}
        </span>
        {complete ? (
          <CheckCircle2 className="onboarding-step-check" aria-hidden="true" />
        ) : null}
      </header>
      {current || !locked ? (
        <div className="onboarding-step-content">{children}</div>
      ) : null}
    </section>
  );
}
