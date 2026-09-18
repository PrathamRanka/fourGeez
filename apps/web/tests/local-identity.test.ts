import { describe, expect, it } from "vitest";
import { createLocalIdentityAdapter } from "@/features/auth/server/local-identity-adapter";

describe("local seller identity adapter", () => {
  it("supports registration, verification, sign-in, recovery, and revocation", async () => {
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
