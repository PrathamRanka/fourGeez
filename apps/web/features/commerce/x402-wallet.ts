const baseSepoliaNetwork = "eip155:84532";
const baseSepoliaChainHex = "0x14a34";
const baseSepoliaUSDC = "0x036CbD53842c5426634e7929541eC2318f3dCF7e";
const ethereumAddressPattern = /^0x[0-9a-fA-F]{40}$/;

export type EIP1193Provider = {
  request(input: { method: string; params?: unknown[] }): Promise<unknown>;
};

type PaymentRequirements = {
  scheme: "exact";
  network: string;
  asset: string;
  amount: string;
  payTo: string;
  maxTimeoutSeconds: number;
  extra?: { name?: string; version?: string };
};

type ResourceInfo = {
  url: string;
  description?: string;
  mimeType?: string;
};

type MockPaymentRequirements = PaymentRequirements & {
  resource: string;
  description?: string;
  mimeType?: string;
};

export type DecodedPaymentRequired =
  | {
      mode: "mock";
      requirements: PaymentRequirements;
      resource: ResourceInfo;
    }
  | {
      mode: "x402";
      requirements: PaymentRequirements;
      resource: ResourceInfo;
      raw: {
        x402Version: 2;
        accepts: PaymentRequirements[];
        resource: ResourceInfo;
        extensions?: Record<string, unknown>;
      };
    };

export function getInjectedWallet(): EIP1193Provider | null {
  if (typeof window === "undefined") {
    return null;
  }
  return (window as Window & { ethereum?: EIP1193Provider }).ethereum ?? null;
}

export function decodePaymentRequired(
  encodedChallenge: string,
): DecodedPaymentRequired {
  let decoded: unknown;
  try {
    decoded = JSON.parse(decodeBase64(encodedChallenge));
  } catch {
    throw new Error("AgentPay returned an invalid x402 payment challenge.");
  }
  if (!isRecord(decoded)) {
    throw new Error("AgentPay returned an invalid x402 payment challenge.");
  }

  if (decoded.x402Version === 2) {
    const accepts = Array.isArray(decoded.accepts) ? decoded.accepts : [];
    const requirements = accepts.find(isSupportedRequirements);
    if (!requirements || !isResource(decoded.resource)) {
      throw new Error(
        "This wallet supports only exact Base Sepolia x402 payments.",
      );
    }
    return {
      mode: "x402",
      requirements,
      resource: decoded.resource,
      raw: {
        x402Version: 2,
        accepts: accepts.filter(isPaymentRequirements),
        resource: decoded.resource,
        ...(isRecord(decoded.extensions)
          ? { extensions: decoded.extensions }
          : {}),
      },
    };
  }

  if (
    isMockRequirements(decoded) &&
    typeof decoded.resource === "string" &&
    decoded.resource.length > 0
  ) {
    return {
      mode: "mock",
      requirements: decoded,
      resource: {
        url: decoded.resource,
        description:
          typeof decoded.description === "string"
            ? decoded.description
            : undefined,
        mimeType:
          typeof decoded.mimeType === "string" ? decoded.mimeType : undefined,
      },
    };
  }

  throw new Error(
    "This wallet supports only exact Base Sepolia x402 payments.",
  );
}

function isMockRequirements(value: unknown): value is MockPaymentRequirements {
  if (!isPaymentRequirements(value)) {
    return false;
  }
  const candidate = value as PaymentRequirements & Record<string, unknown>;
  return (
    candidate.network === baseSepoliaNetwork &&
    candidate.asset.length > 0 &&
    candidate.payTo.length > 0 &&
    typeof candidate.resource === "string" &&
    candidate.resource.length > 0
  );
}

export async function createX402PaymentSignature(
  encodedChallenge: string,
  provider: EIP1193Provider,
  nowSeconds = Math.floor(Date.now() / 1000),
): Promise<string> {
  const challenge = decodePaymentRequired(encodedChallenge);
  if (challenge.mode !== "x402") {
    throw new Error("A wallet signature is not required for local mock mode.");
  }
  const [account] = asStringArray(
    await provider.request({ method: "eth_requestAccounts" }),
  );
  if (!account || !ethereumAddressPattern.test(account)) {
    throw new Error("Connect an EVM wallet to continue.");
  }

  await provider.request({
    method: "wallet_switchEthereumChain",
    params: [{ chainId: baseSepoliaChainHex }],
  });

  const requirements = challenge.requirements;
  const authorization = {
    from: account,
    to: requirements.payTo,
    value: requirements.amount,
    validAfter: String(nowSeconds),
    validBefore: String(nowSeconds + 3600),
    nonce: randomNonce(),
  };
  const typedData = {
    domain: {
      name: requirements.extra?.name ?? "USDC",
      version: requirements.extra?.version ?? "2",
      chainId: 84532,
      verifyingContract: assetContractAddress(requirements.asset),
    },
    types: {
      EIP712Domain: [
        { name: "name", type: "string" },
        { name: "version", type: "string" },
        { name: "chainId", type: "uint256" },
        { name: "verifyingContract", type: "address" },
      ],
      TransferWithAuthorization: [
        { name: "from", type: "address" },
        { name: "to", type: "address" },
        { name: "value", type: "uint256" },
        { name: "validAfter", type: "uint256" },
        { name: "validBefore", type: "uint256" },
        { name: "nonce", type: "bytes32" },
      ],
    },
    primaryType: "TransferWithAuthorization",
    message: authorization,
  };
  const signature = await provider.request({
    method: "eth_signTypedData_v4",
    params: [account, JSON.stringify(typedData)],
  });
  if (typeof signature !== "string" || !signature.startsWith("0x")) {
    throw new Error("The wallet did not return a valid payment signature.");
  }

  return encodeBase64(
    JSON.stringify({
      x402Version: 2,
      payload: { signature, authorization },
      accepted: requirements,
      resource: challenge.resource,
      ...(challenge.raw.extensions
        ? { extensions: challenge.raw.extensions }
        : {}),
    }),
  );
}

function isSupportedRequirements(value: unknown): value is PaymentRequirements {
  return (
    isPaymentRequirements(value) &&
    value.scheme === "exact" &&
    value.network === baseSepoliaNetwork &&
    (ethereumAddressPattern.test(value.asset) || value.asset === "USDC") &&
    ethereumAddressPattern.test(value.payTo)
  );
}

function assetContractAddress(asset: string): string {
  if (asset === "USDC") {
    return baseSepoliaUSDC;
  }
  if (ethereumAddressPattern.test(asset)) {
    return asset;
  }
  throw new Error("This wallet does not support the requested payment asset.");
}

function isPaymentRequirements(value: unknown): value is PaymentRequirements {
  return (
    isRecord(value) &&
    value.scheme === "exact" &&
    typeof value.network === "string" &&
    typeof value.asset === "string" &&
    typeof value.amount === "string" &&
    /^\d+$/.test(value.amount) &&
    typeof value.payTo === "string" &&
    Number.isInteger(value.maxTimeoutSeconds) &&
    Number(value.maxTimeoutSeconds) > 0
  );
}

function isResource(value: unknown): value is ResourceInfo {
  return (
    isRecord(value) && typeof value.url === "string" && value.url.length > 0
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((entry): entry is string => typeof entry === "string")
    : [];
}

function randomNonce(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return `0x${Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("")}`;
}

function decodeBase64(value: string): string {
  const binary = atob(value);
  return new TextDecoder().decode(
    Uint8Array.from(binary, (character) => character.charCodeAt(0)),
  );
}

function encodeBase64(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}
