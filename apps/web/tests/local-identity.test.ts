import { createHmac } from "node:crypto";
import { describe, expect, it } from "vitest";
import { createLocalIdentityAdapter } from "@/features/auth/server/local-identity-adapter";

describe("local seller identity adapter", () => {
  it("seeds a verified launch-ready seller account for local dashboard review", async () => {
    const identity = createLocalIdentityAdapter({
      sellerTokenSigningSecret: "test-local-identity-signing-secret-32-bytes",
      seedAccount: {
        email: "pratham@agentpay.local",
        name: "Pratham",
        onboardingComplete: true,
        password: "AgentPayLocalDemo2026",
        sellerId: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
        storefront: {
          sellerId: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
          name: "Northstar Research",
          slug: "demo-seller",
          upstreamBaseUrl: "http://127.0.0.1:8090",
          status: "active",
          version: 3,
        },
        subject: "local-seller",
      },
    });

    const authentication = await identity.signIn({
      email: "pratham@agentpay.local",
      password: "AgentPayLocalDemo2026",
    });

    expect(authentication).toMatchObject({
      ok: true,
      value: {
        principal: {
          subject: "local-seller",
          sellerId: "sel_01K5D09YJ0C0M7RJM4FWQ0K9H7",
          onboardingComplete: true,
          storefront: { slug: "demo-seller", status: "active" },
        },
      },
    });
  });

  it("supports static-token compatibility for existing local launchers", async () => {
    const identity = createLocalIdentityAdapter({
      sellerAccessToken: "local-seller-token",
    });

    const registration = await identity.signUp({
      email: "Owner@Example.com",
      name: "Northstar Research",
      password: "correct-horse-battery-staple",
    });
    expect(registration.ok).toBe(true);
    if (!registration.ok) return;

    const beforeVerification = await identity.signIn({
      email: "owner@example.com",
      password: "correct-horse-battery-staple",
    });
    expect(beforeVerification).toMatchObject({
      ok: false,
      code: "verification_required",
    });

    expect(
      await identity.verifyRegistration({
        challengeId: registration.value.challengeId,
        code: registration.value.developmentCode,
      }),
    ).toEqual({ ok: true, value: undefined });

    const authenticated = await identity.signIn({
      email: "owner@example.com",
      password: "correct-horse-battery-staple",
    });
    expect(authenticated).toMatchObject({
      ok: true,
      value: {
        accessToken: "local-seller-token",
        principal: {
          email: "owner@example.com",
          name: "Northstar Research",
          sellerId: null,
          onboardingComplete: false,
        },
      },
    });
    if (!authenticated.ok) return;

    const recovery = await identity.beginRecovery({
      email: "owner@example.com",
    });
    expect(recovery.ok).toBe(true);
    if (!recovery.ok) return;
    await identity.completeRecovery({
      challengeId: recovery.value.challengeId,
      code: recovery.value.developmentCode,
      password: "new-correct-horse-battery-staple",
    });

    expect(
      await identity.signIn({
        email: "owner@example.com",
        password: "new-correct-horse-battery-staple",
      }),
    ).toMatchObject({ ok: true });

    await identity.revoke(authenticated.value.accessToken);
    expect(await identity.validate(authenticated.value.accessToken)).toEqual({
      ok: false,
      code: "session_revoked",
      error: "Your session has ended. Sign in again.",
    });
  });

  it("mints a unique signed bearer and subject for every local account", async () => {
    const signingSecret = "test-local-identity-signing-secret-32-bytes";
    const identity = createLocalIdentityAdapter({
      sellerTokenSigningSecret: signingSecret,
    });

    const first = await registerAndSignIn(
      identity,
      "first@example.com",
      "First Seller",
    );
    const second = await registerAndSignIn(
      identity,
      "second@example.com",
      "Second Seller",
    );

    const firstClaims = readSignedClaims(first.accessToken, signingSecret);
    const secondClaims = readSignedClaims(second.accessToken, signingSecret);

    expect(first.accessToken).not.toBe(second.accessToken);
    expect(first.principal.subject).not.toBe(second.principal.subject);
    expect(firstClaims.subject).toBe(first.principal.subject);
    expect(secondClaims.subject).toBe(second.principal.subject);
    expect(firstClaims.tokenId).not.toBe(secondClaims.tokenId);
    expect(firstClaims.sessionId).not.toBe(secondClaims.sessionId);
    expect(firstClaims.expiresAt - firstClaims.issuedAt).toBe(60 * 60);
  });

  it("rejects a weak local signing secret", () => {
    expect(() =>
      createLocalIdentityAdapter({ sellerTokenSigningSecret: "too-short" }),
    ).toThrow("32 to 4096 bytes");
  });

  it("returns the same recovery response for unknown accounts", async () => {
    const identity = createLocalIdentityAdapter({
      sellerAccessToken: "local-seller-token",
    });
    const result = await identity.beginRecovery({
      email: "missing@example.com",
    });
    expect(result).toMatchObject({ ok: true });
  });
});

async function registerAndSignIn(
  identity: ReturnType<typeof createLocalIdentityAdapter>,
  email: string,
  name: string,
) {
  const password = "correct-horse-battery-staple";
  const registration = await identity.signUp({ email, name, password });
  if (!registration.ok) throw new Error(registration.error);
  const verification = await identity.verifyRegistration({
    challengeId: registration.value.challengeId,
    code: registration.value.developmentCode,
  });
  if (!verification.ok) throw new Error(verification.error);
  const authentication = await identity.signIn({ email, password });
  if (!authentication.ok) throw new Error(authentication.error);
  return authentication.value;
}

function readSignedClaims(accessToken: string, signingSecret: string) {
  const parts = accessToken.split(".");
  expect(parts).toHaveLength(7);
  const unsignedToken = parts.slice(0, 6).join(".");
  expect(parts[6]).toBe(
    createHmac("sha256", signingSecret)
      .update(unsignedToken)
      .digest("base64url"),
  );
  return {
    subject: Buffer.from(parts[1], "base64url").toString("utf8"),
    tokenId: Buffer.from(parts[2], "base64url").toString("utf8"),
    sessionId: Buffer.from(parts[3], "base64url").toString("utf8"),
    issuedAt: Number(parts[4]),
    expiresAt: Number(parts[5]),
  };
}
