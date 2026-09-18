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
import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import type {
  CredentialCreated,
  OnboardingActions,
  OnboardingSnapshot,
  Seller,
} from "@/features/onboarding/model";
import {
  createMCPConfiguration,
  setupPrompt,
} from "@/features/onboarding/model";

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

const defaultAsset = "USDC";
const defaultNetwork = "eip155:84532";

// SellerOnboarding coordinates the five seller-controlled launch prerequisites.
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
  const [createdCredential, setCreatedCredential] =
    useState<CredentialCreated | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [pendingStep, setPendingStep] = useState<string | null>(null);
  const activeDestination = paymentDestinations.find(
    (destination) => destination.status === "active",
  );
  const activeCredential = credentials.find(
    (credential) => !credential.revokedAt,
  );
  const credentialReady = Boolean(activeCredential || createdCredential);
  const completedSteps =
    1 +
    Number(Boolean(seller)) +
    Number(Boolean(activeDestination)) +
    Number(credentialReady);
  const mcpConfiguration = useMemo(
    () =>
      createdCredential
        ? createMCPConfiguration(apiOrigin, createdCredential.token)
        : "",
    [apiOrigin, createdCredential],
  );

  // submitStorefront creates the first seller-owned resource through the server controller.
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
    window.history.replaceState(
      null,
      "",
      `/dashboard/onboarding?sellerId=${encodeURIComponent(result.value.sellerId)}`,
    );
  }

  // connectWallet requests an address and signature without exposing private key material.
  async function connectWallet() {
    if (!seller) {
      return;
    }
    if (!window.ethereum) {
      setErrorMessage(
        "Install or open an EVM-compatible browser wallet to continue.",
      );
      return;
    }

    setErrorMessage(null);
    setPendingStep("wallet");
    try {
      const accountResult = await window.ethereum.request({
        method: "eth_requestAccounts",
      });
      const address = Array.isArray(accountResult)
        ? String(accountResult[0] ?? "")
        : "";
      if (!address) {
        throw new Error("The wallet did not return an address.");
      }

      const preparedResult = await actions.preparePaymentDestination({
        sellerId: seller.sellerId,
        asset: defaultAsset,
        network: defaultNetwork,
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

      setPaymentDestinations((currentDestinations) => [
        ...currentDestinations.filter(
          (destination) =>
            destination.destinationId !==
            verificationResult.value.destinationId,
        ),
        verificationResult.value,
      ]);
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

  // issueCredential creates one scoped token and retains it only in current browser memory.
  async function issueCredential() {
    if (!seller) {
      return;
    }
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
    setCredentials((currentCredentials) => [
      ...currentCredentials,
      result.value,
    ]);
  }

  // copyText writes one setup artifact after an explicit user action.
  async function copyText(value: string) {
    await navigator.clipboard.writeText(value);
  }

  return (
    <div className="onboarding-layout">
      <aside className="onboarding-summary">
        <p className="dashboard-eyebrow">Launch Rail</p>
        <h1>Launch your storefront</h1>
        <p>
          Connect the minimum required pieces. AgentPay keeps payment,
          publishing, and production changes behind your approval.
        </p>
        <div className="onboarding-progress" aria-label="Onboarding progress">
          <div>
            <span>{completedSteps} of 5 complete</span>
            <span>{completedSteps * 20}%</span>
          </div>
          <div className="onboarding-progress-track" aria-hidden="true">
            <span style={{ transform: `scaleX(${completedSteps / 5})` }} />
          </div>
        </div>
        <OnboardingChecklist
          sellerReady={Boolean(seller)}
          destinationReady={Boolean(activeDestination)}
          credentialReady={credentialReady}
        />
      </aside>

      <div className="onboarding-steps">
        {errorMessage ? (
          <div className="dashboard-error" role="alert">
            {errorMessage}
          </div>
        ) : null}

        <OnboardingStep
          number="01"
          icon={Store}
          title="Create your storefront"
          description="Name the seller experience and point AgentPay at the HTTPS service that fulfills purchases."
          complete={Boolean(seller)}
        >
          {seller ? (
            <div className="onboarding-complete-row">
              <div>
                <strong>Storefront created</strong>
                <span>{seller.slug}</span>
              </div>
              <CheckCircle2 aria-hidden="true" />
            </div>
          ) : (
            <form
              aria-label="Create storefront"
              className="onboarding-form"
              onSubmit={submitStorefront}
            >
              <label>
                <span>Storefront name</span>
                <input
                  name="name"
                  required
                  maxLength={120}
                  placeholder="Northstar Research"
                />
              </label>
              <label>
                <span>Storefront slug</span>
                <input
                  name="slug"
                  required
                  minLength={3}
                  maxLength={48}
                  pattern="[a-z0-9-]+"
                  placeholder="northstar-research"
                />
              </label>
              <label className="onboarding-field-wide">
                <span>Upstream API URL</span>
                <input
                  name="upstreamBaseUrl"
                  required
                  type="url"
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
          )}
        </OnboardingStep>

        <OnboardingStep
          number="02"
          icon={WalletCards}
          title="Verify your payment destination"
          description="AgentPay asks your wallet to sign a one-time challenge. Private keys never leave the wallet."
          complete={Boolean(activeDestination)}
        >
          {activeDestination ? (
            <div className="onboarding-complete-row">
              <div>
                <strong>Payment destination verified</strong>
                <span>{activeDestination.address}</span>
              </div>
              <CheckCircle2 aria-hidden="true" />
            </div>
          ) : (
            <div className="onboarding-action-row">
              <div>
                <strong>USDC on Base Sepolia</strong>
                <span>V1 testnet destination</span>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={!seller || pendingStep === "wallet"}
                onClick={connectWallet}
              >
                {pendingStep === "wallet" ? (
                  <LoaderCircle className="animate-spin" aria-hidden="true" />
                ) : null}
                Connect browser wallet
              </Button>
            </div>
          )}
        </OnboardingStep>

        <OnboardingStep
          number="03"
          icon={KeyRound}
          title="Create a project key"
          description="The key is scoped to this seller and grants only the setup operations your coding agent needs."
          complete={credentialReady}
        >
          {createdCredential ? (
            <div className="credential-secret">
              <strong>Save this key now. It is shown only once.</strong>
              <code>{createdCredential.token}</code>
            </div>
          ) : activeCredential ? (
            <div className="onboarding-complete-row">
              <div>
                <strong>Coding agent connected</strong>
                <span>{activeCredential.label}</span>
              </div>
              <CheckCircle2 aria-hidden="true" />
            </div>
          ) : (
            <Button
              type="button"
              disabled={!activeDestination || pendingStep === "credential"}
              onClick={issueCredential}
            >
              {pendingStep === "credential" ? (
                <LoaderCircle className="animate-spin" aria-hidden="true" />
              ) : null}
              Create integration key
            </Button>
          )}
        </OnboardingStep>

        <OnboardingStep
          number="04"
          icon={Code2}
          title="Connect your coding agent"
          description="Copy the MCP configuration and the setup prompt into Claude Code, Codex, or another MCP host."
          complete={credentialReady}
        >
          {createdCredential ? (
            <div className="onboarding-copy-grid">
              <div className="onboarding-code-block">
                <div>
                  <span>MCP configuration</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    aria-label="Copy MCP configuration"
                    onClick={() => copyText(mcpConfiguration)}
                  >
                    <Clipboard aria-hidden="true" />
                    Copy
                  </Button>
                </div>
                <pre>{mcpConfiguration}</pre>
              </div>
              <div className="onboarding-code-block">
                <div>
                  <span>Setup prompt</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    aria-label="Copy setup prompt"
                    onClick={() => copyText(setupPrompt)}
                  >
                    <Clipboard aria-hidden="true" />
                    Copy
                  </Button>
                </div>
                <p>{setupPrompt}</p>
              </div>
            </div>
          ) : (
            <p className="onboarding-muted-state">
              Create a project key to reveal setup instructions.
            </p>
          )}
        </OnboardingStep>

        <OnboardingStep
          number="05"
          icon={ShieldCheck}
          title="Run the sandbox purchase"
          description="The coding agent validates discovery, payment gating, signed forwarding, and exactly-once fulfillment before publication."
          complete={false}
        >
          <div className="sandbox-readiness">
            <span
              className={credentialReady ? "status-ready" : "status-waiting"}
            >
              {credentialReady
                ? "Ready for product validation"
                : "Waiting for setup"}
            </span>
            <p>
              Product validation becomes available after your coding agent
              proposes the first paid route.
            </p>
          </div>
        </OnboardingStep>
      </div>
    </div>
  );
}

type OnboardingChecklistProps = {
  credentialReady: boolean;
  destinationReady: boolean;
  sellerReady: boolean;
};

// OnboardingChecklist keeps all prerequisites visible while the seller moves between steps.
function OnboardingChecklist({
  credentialReady,
  destinationReady,
  sellerReady,
}: OnboardingChecklistProps) {
  const items = [
    { label: "Seller account ready", complete: true },
    { label: "Storefront created", complete: sellerReady },
    { label: "Payment destination verified", complete: destinationReady },
    { label: "Coding agent connected", complete: credentialReady },
    { label: "Sandbox purchase passed", complete: false },
  ];

  return (
    <ul className="onboarding-checklist" aria-label="Onboarding checklist">
      {items.map((item) => (
        <li key={item.label} data-complete={item.complete}>
          {item.complete ? (
            <Check aria-hidden="true" />
          ) : (
            <Circle aria-hidden="true" />
          )}
          {item.label}
        </li>
      ))}
    </ul>
  );
}

type OnboardingStepProps = {
  children: React.ReactNode;
  complete: boolean;
  description: string;
  icon: React.ComponentType<{
    className?: string;
    "aria-hidden"?: boolean | "true" | "false";
  }>;
  number: string;
  title: string;
};

// OnboardingStep renders one numbered setup boundary without hiding later requirements.
function OnboardingStep({
  children,
  complete,
  description,
  icon: Icon,
  number,
  title,
}: OnboardingStepProps) {
  return (
    <section className="onboarding-step" data-complete={complete}>
      <header>
        <span className="onboarding-step-number">{number}</span>
        <span className="onboarding-step-icon">
          <Icon className="size-4" aria-hidden="true" />
        </span>
        <div>
          <h2>{title}</h2>
          <p>{description}</p>
        </div>
        {complete ? (
          <CheckCircle2 className="onboarding-step-check" aria-hidden="true" />
        ) : null}
      </header>
      <div className="onboarding-step-content">{children}</div>
    </section>
  );
}
