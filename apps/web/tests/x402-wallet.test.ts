import { describe, expect, it, vi } from "vitest";
import {
  decodePaymentRequired,
  createX402PaymentSignature,
  detectWalletCompatibility,
  PaymentCapabilityError,
} from "@/features/commerce/x402-wallet";

const challenge = {
  x402Version: 2,
  accepts: [
    {
      scheme: "exact",
      network: "eip155:84532",
      asset: "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
      amount: "35000000",
      payTo: "0x1111111111111111111111111111111111111111",
      maxTimeoutSeconds: 60,
      extra: { name: "USDC", version: "2" },
    },
  ],
  resource: {
    url: "https://api.agentpay.example/pay/northstar/research/basic",
    description: "Generate a report",
    mimeType: "application/json",
  },
};

describe("x402 wallet", () => {
  it("decodes and validates an exact Base Sepolia challenge", () => {
    const decoded = decodePaymentRequired(btoa(JSON.stringify(challenge)));
    expect(decoded.mode).toBe("x402");
    expect(decoded.requirements.amount).toBe("35000000");
  });

  it("creates the bound EIP-3009 x402 v2 payload", async () => {
    const request = vi
      .fn()
      .mockResolvedValueOnce(["0x2222222222222222222222222222222222222222"])
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce("0xsigned");
    const provider = { request };

    const signature = await createX402PaymentSignature(
      btoa(JSON.stringify(challenge)),
      provider,
      1_758_200_000,
    );
    const payload = JSON.parse(atob(signature));

    expect(payload).toMatchObject({
      x402Version: 2,
      accepted: challenge.accepts[0],
      resource: challenge.resource,
      payload: {
        signature: "0xsigned",
        authorization: {
          from: "0x2222222222222222222222222222222222222222",
          to: challenge.accepts[0].payTo,
          value: "35000000",
          validAfter: "0",
          validBefore: "1758200060",
        },
      },
    });
    expect(request).toHaveBeenNthCalledWith(2, {
      method: "wallet_switchEthereumChain",
      params: [{ chainId: "0x14a34" }],
    });
    expect(request).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({ method: "eth_signTypedData_v4" }),
    );
    expect(payload.payload.authorization.nonce).toMatch(/^0x[0-9a-f]{64}$/);
  });

  it("detects wallet compatibility without requesting a signature", async () => {
    await expect(detectWalletCompatibility(null)).resolves.toEqual({
      compatible: false,
      code: "wallet_missing",
      message: "Install or open an EVM wallet to continue.",
      recoveryAction: "connect_wallet",
    });

    const disconnected = {
      request: vi
        .fn()
        .mockResolvedValueOnce("0x14a34")
        .mockResolvedValueOnce([]),
    };
    await expect(detectWalletCompatibility(disconnected)).resolves.toEqual({
      compatible: false,
      code: "wallet_disconnected",
      message: "Connect your wallet account to continue.",
      recoveryAction: "connect_wallet",
    });

    const wrongNetwork = {
      request: vi
        .fn()
        .mockResolvedValueOnce("0x1")
        .mockResolvedValueOnce(["0x2222222222222222222222222222222222222222"]),
    };
    await expect(detectWalletCompatibility(wrongNetwork)).resolves.toEqual({
      compatible: false,
      code: "network_switch_required",
      message: "Switch the wallet to Base Sepolia to continue.",
      recoveryAction: "switch_network",
    });
  });

  it("classifies unsupported network switching and typed-data signing", async () => {
    const switchUnsupported = {
      request: vi
        .fn()
        .mockResolvedValueOnce(["0x2222222222222222222222222222222222222222"])
        .mockRejectedValueOnce({ code: 4902 }),
    };
    await expect(
      createX402PaymentSignature(
        btoa(JSON.stringify(challenge)),
        switchUnsupported,
        1_758_200_000,
      ),
    ).rejects.toMatchObject<Partial<PaymentCapabilityError>>({
      code: "network_unsupported",
      recoveryAction: "switch_network",
    });

    const signingUnsupported = {
      request: vi
        .fn()
        .mockResolvedValueOnce(["0x2222222222222222222222222222222222222222"])
        .mockResolvedValueOnce(null)
        .mockRejectedValueOnce({ code: -32601 }),
    };
    await expect(
      createX402PaymentSignature(
        btoa(JSON.stringify(challenge)),
        signingUnsupported,
        1_758_200_000,
      ),
    ).rejects.toMatchObject<Partial<PaymentCapabilityError>>({
      code: "typed_data_unsupported",
      recoveryAction: "connect_wallet",
    });
  });

  it("rejects a non-exact or unsupported challenge", () => {
    expect(() =>
      decodePaymentRequired(
        btoa(
          JSON.stringify({
            ...challenge,
            accepts: [{ ...challenge.accepts[0], scheme: "upto" }],
          }),
        ),
      ),
    ).toThrow(/supports only exact Base Sepolia/i);

    expect(() =>
      decodePaymentRequired(
        btoa(
          JSON.stringify({
            ...challenge,
            accepts: [
              {
                ...challenge.accepts[0],
                asset: "0x1111111111111111111111111111111111111111",
              },
            ],
          }),
        ),
      ),
    ).toThrow(/supports only exact Base Sepolia/i);
  });
});
