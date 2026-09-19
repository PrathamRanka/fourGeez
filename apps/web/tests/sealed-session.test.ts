import { createCipheriv } from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  openSellerSession,
  sealSellerSession,
} from "@/features/auth/server/sealed-session";

const encryptionKey = Buffer.alloc(32, 7).toString("base64url");
const practicalBrowserCookieLimitBytes = 4_096;

function deterministicToken(length: number): string {
  let token = "";
  let state = 0x6d2b79f5;
  const alphabet =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
  while (token.length < length) {
    state = Math.imul(state ^ (state >>> 15), state | 1);
    state ^= state + Math.imul(state ^ (state >>> 7), state | 61);
    state ^= state >>> 14;
    token += alphabet[state & 63];
  }
  return token;
}

function sealLegacySession(
  authentication: Parameters<typeof sealSellerSession>[0],
): string {
  const initializationVector = Buffer.alloc(12, 3);
  const cipher = createCipheriv(
    "aes-256-gcm",
    Buffer.from(encryptionKey, "base64url"),
    initializationVector,
  );
  cipher.setAAD(Buffer.from("agentpay-seller-session-v1", "utf8"));
  const ciphertext = Buffer.concat([
    cipher.update(Buffer.from(JSON.stringify(authentication), "utf8")),
    cipher.final(),
  ]);
  return [
    "aps1",
    initializationVector.toString("base64url"),
    ciphertext.toString("base64url"),
    cipher.getAuthTag().toString("base64url"),
  ].join(".");
}

describe("sealed production seller sessions", () => {
  it("survives process boundaries without exposing bearer or refresh tokens", () => {
    const sealed = sealSellerSession(
      {
        accessToken: "server-only-access-token",
        refreshToken: "server-only-refresh-token",
        expiresAt: "2026-09-19T11:00:00.000Z",
        sessionExpiresAt: "2026-09-19T18:00:00.000Z",
        principal: {
          subject: "seller-subject",
          email: "owner@example.com",
          name: "Northstar Research",
          sellerId: "sel_123",
          onboardingComplete: true,
        },
      },
      encryptionKey,
    );

    expect(sealed).not.toContain("server-only-access-token");
    expect(sealed).not.toContain("server-only-refresh-token");
    expect(
      openSellerSession(
        sealed,
        encryptionKey,
        Date.parse("2026-09-19T10:30:00Z"),
      ),
    ).toMatchObject({
      accessToken: "server-only-access-token",
      principal: { sellerId: "sel_123" },
    });
  });

  it("opens legacy aps1 sessions during a rolling deployment", () => {
    const authentication = {
      accessToken: "legacy-access-token",
      refreshToken: "legacy-refresh-token",
      expiresAt: "2026-09-19T11:00:00.000Z",
      sessionExpiresAt: "2026-09-19T18:00:00.000Z",
      principal: {
        subject: "legacy-seller-subject",
        email: "legacy@example.com",
        name: "Legacy Seller",
        sellerId: "sel_legacy",
        onboardingComplete: true,
      },
    };

    expect(
      openSellerSession(
        sealLegacySession(authentication),
        encryptionKey,
        Date.parse("2026-09-19T10:30:00Z"),
      ),
    ).toEqual(authentication);
  });

  it("rejects tampering, wrong keys, and expired sessions", () => {
    const sealed = sealSellerSession(
      {
        accessToken: "server-only-access-token",
        expiresAt: "2026-09-19T11:00:00.000Z",
        principal: {
          subject: "seller-subject",
          email: "owner@example.com",
          name: "Owner",
          sellerId: null,
          onboardingComplete: false,
        },
      },
      encryptionKey,
    );

    const ciphertextStart = sealed.indexOf(".") + 1;
    const tamperIndex = sealed.indexOf(".", ciphertextStart) + 2;
    const tampered = `${sealed.slice(0, tamperIndex)}${
      sealed[tamperIndex] === "a" ? "b" : "a"
    }${sealed.slice(tamperIndex + 1)}`;
    expect(
      openSellerSession(
        tampered,
        encryptionKey,
        Date.parse("2026-09-19T10:30:00Z"),
      ),
    ).toBeNull();
    expect(
      openSellerSession(
        sealed,
        Buffer.alloc(32, 8).toString("base64url"),
        Date.parse("2026-09-19T10:30:00Z"),
      ),
    ).toBeNull();
    expect(
      openSellerSession(
        sealed,
        encryptionKey,
        Date.parse("2026-09-19T18:00:00Z"),
      ),
    ).toBeNull();
  });

  it("keeps a production cookie below browser limits with realistic Cognito tokens", () => {
    const sealed = sealSellerSession(
      {
        accessToken: deterministicToken(1_070),
        refreshToken: deterministicToken(1_782),
        expiresAt: "2026-09-20T11:00:00.000Z",
        sessionExpiresAt: "2026-09-20T18:00:00.000Z",
        principal: {
          subject: "41c32d7a-50a1-7028-b8bc-fad70e17053c",
          email: "seller-live-test@example.com",
          name: "AgentPay Live Test Seller",
          sellerId: "sel_01K5JQ4P6B4G6W8JAV9KQ1H4NM",
          onboardingComplete: true,
          storefront: {
            sellerId: "sel_01K5JQ4P6B4G6W8JAV9KQ1H4NM",
            name: "AgentPay Live Test Seller",
            slug: "agentpay-live-test-seller",
            upstreamBaseUrl:
              "https://agentpay-live-seller-prototype.vercel.app",
            status: "active",
            version: 3,
          },
        },
      },
      encryptionKey,
    );
    const serializedCookie = `__Host-agentpay_seller_session=${sealed}; Path=/; Max-Age=28800; HttpOnly; Secure; SameSite=Strict`;

    expect(Buffer.byteLength(serializedCookie, "utf8")).toBeLessThan(
      practicalBrowserCookieLimitBytes,
    );
  });
});
