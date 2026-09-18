import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const signingSecret = "test-local-identity-signing-secret-32-bytes";

describe("seller identity selection", () => {
  beforeEach(() => {
    vi.resetModules();
    delete globalThis.agentPayLocalIdentity;
    process.env.AGENTPAY_ENV = "local";
    process.env.AGENTPAY_IDENTITY_MODE = "local";
    delete process.env.AGENTPAY_SELLER_BEARER_TOKEN;
  });

  afterEach(() => {
    delete globalThis.agentPayLocalIdentity;
    delete process.env.AGENTPAY_ENV;
    delete process.env.AGENTPAY_IDENTITY_MODE;
    delete process.env.AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET;
    delete process.env.AGENTPAY_SELLER_BEARER_TOKEN;
  });

  it("uses the environment-provided signing secret in local mode", async () => {
    process.env.AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET = signingSecret;
    const { getIdentityAdapter } =
      await import("@/features/auth/server/identity");
    const identity = getIdentityAdapter();
    const registration = await identity.signUp({
      email: "owner@example.com",
      name: "Owner",
      password: "correct-horse-battery-staple",
    });
    expect(registration.ok).toBe(true);
    if (!registration.ok) return;
    await identity.verifyRegistration({
      challengeId: registration.value.challengeId,
      code: registration.value.developmentCode ?? "",
    });
    const authentication = await identity.signIn({
      email: "owner@example.com",
      password: "correct-horse-battery-staple",
    });
    expect(authentication).toMatchObject({ ok: true });
    if (authentication.ok) {
      expect(authentication.value.accessToken).toMatch(/^apls1\./);
    }
  });

  it("does not activate local identity in production mode", async () => {
    process.env.AGENTPAY_ENV = "prod";
    process.env.AGENTPAY_IDENTITY_MODE = "cognito";
    process.env.AGENTPAY_LOCAL_IDENTITY_SIGNING_SECRET = signingSecret;
    const { getIdentityAdapter } =
      await import("@/features/auth/server/identity");
    const result = await getIdentityAdapter().signIn({
      email: "owner@example.com",
      password: "correct-horse-battery-staple",
    });
    expect(result).toMatchObject({
      ok: false,
      code: "dependency_unavailable",
    });
  });
});
