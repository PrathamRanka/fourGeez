import { describe, expect, it } from "vitest";
import {
  openSellerSession,
  sealSellerSession,
} from "@/features/auth/server/sealed-session";

const encryptionKey = Buffer.alloc(32, 7).toString("base64url");

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
});
