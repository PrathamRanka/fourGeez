import { describe, expect, it, vi } from "vitest";
import { createCognitoIdentityAdapter } from "@/features/auth/server/cognito-identity-adapter";

function cognitoClient(responses: Record<string, unknown>) {
  return {
    send: vi.fn(async (command: object) => {
      const commandName = command.constructor.name;
      const response = responses[commandName];
      if (response instanceof Error) throw response;
      return response ?? {};
    }),
  };
}

describe("Cognito seller identity adapter", () => {
  it("registers and verifies an email account without returning a secret", async () => {
    const client = cognitoClient({
      SignUpCommand: { UserSub: "seller-subject" },
      ConfirmSignUpCommand: {},
      ResendConfirmationCodeCommand: {},
    });
    const identity = createCognitoIdentityAdapter({
      client,
      clientId: "seller-client",
      now: () => Date.parse("2026-09-19T10:00:00Z"),
    });

    const registration = await identity.signUp({
      email: " Owner@Example.com ",
      name: "Owner",
      password: "correct-horse-battery-staple",
    });
    expect(registration).toEqual({
      ok: true,
      value: { challengeId: "owner@example.com" },
    });
    expect(client.send).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        input: expect.objectContaining({
          ClientId: "seller-client",
          Username: "owner@example.com",
        }),
      }),
    );

    await expect(
      identity.verifyRegistration({
        challengeId: "owner@example.com",
        code: "123456",
      }),
    ).resolves.toEqual({ ok: true, value: undefined });
    await expect(
      identity.resendVerification?.({ email: "owner@example.com" }),
    ).resolves.toEqual({
      ok: true,
      value: { challengeId: "owner@example.com" },
    });
  });

  it("signs in and derives the seller principal from Cognito attributes", async () => {
    const client = cognitoClient({
      InitiateAuthCommand: {
        AuthenticationResult: {
          AccessToken: "cognito-access-token",
          ExpiresIn: 3600,
          RefreshToken: "cognito-refresh-token",
        },
      },
      GetUserCommand: {
        Username: "owner@example.com",
        UserAttributes: [
          { Name: "sub", Value: "seller-subject" },
          { Name: "email", Value: "owner@example.com" },
          { Name: "name", Value: "Northstar Research" },
        ],
      },
    });
    const identity = createCognitoIdentityAdapter({
      client,
      clientId: "seller-client",
      now: () => Date.parse("2026-09-19T10:00:00Z"),
    });

    const result = await identity.signIn({
      email: "owner@example.com",
      password: "correct-horse-battery-staple",
    });

    expect(result).toEqual({
      ok: true,
      value: {
        accessToken: "cognito-access-token",
        expiresAt: "2026-09-19T11:00:00.000Z",
        refreshToken: "cognito-refresh-token",
        sessionExpiresAt: "2026-09-19T18:00:00.000Z",
        principal: {
          subject: "seller-subject",
          email: "owner@example.com",
          name: "Northstar Research",
          sellerId: null,
          onboardingComplete: false,
        },
      },
    });
  });

  it("uses non-enumerating recovery and credential failure messages", async () => {
    const hiddenUserError = Object.assign(new Error("not found"), {
      name: "UserNotFoundException",
    });
    const client = cognitoClient({
      ForgotPasswordCommand: hiddenUserError,
      InitiateAuthCommand: hiddenUserError,
    });
    const identity = createCognitoIdentityAdapter({
      client,
      clientId: "seller-client",
    });

    await expect(
      identity.beginRecovery({ email: "missing@example.com" }),
    ).resolves.toEqual({
      ok: true,
      value: { challengeId: "missing@example.com" },
    });
    await expect(
      identity.signIn({
        email: "missing@example.com",
        password: "wrong-password",
      }),
    ).resolves.toEqual({
      ok: false,
      code: "invalid_credentials",
      error: "Email or password is incorrect.",
    });
  });

  it("revalidates and globally signs out an authenticated token without losing seller ownership", async () => {
    const client = cognitoClient({
      GetUserCommand: {
        Username: "owner@example.com",
        UserAttributes: [
          { Name: "sub", Value: "seller-subject" },
          { Name: "email", Value: "owner@example.com" },
          { Name: "name", Value: "Northstar Research" },
        ],
      },
      GlobalSignOutCommand: {},
    });
    const identity = createCognitoIdentityAdapter({
      client,
      clientId: "seller-client",
      now: () => Date.parse("2026-09-19T10:00:00Z"),
    });
    const authentication = {
      accessToken: "cognito-access-token",
      expiresAt: "2026-09-19T11:00:00.000Z",
      principal: {
        subject: "seller-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_123",
        onboardingComplete: true,
      },
    };

    await expect(identity.validate(authentication)).resolves.toMatchObject({
      ok: true,
      value: { principal: { sellerId: "sel_123", onboardingComplete: true } },
    });
    await identity.revoke(authentication);
    expect(client.send).toHaveBeenLastCalledWith(
      expect.objectContaining({
        input: { AccessToken: "cognito-access-token" },
      }),
    );
  });

  it("refreshes an expired access token within the absolute seller session", async () => {
    const client = cognitoClient({
      InitiateAuthCommand: {
        AuthenticationResult: {
          AccessToken: "refreshed-access-token",
          ExpiresIn: 3600,
        },
      },
      GetUserCommand: {
        Username: "owner@example.com",
        UserAttributes: [
          { Name: "sub", Value: "seller-subject" },
          { Name: "email", Value: "owner@example.com" },
          { Name: "name", Value: "Northstar Research" },
        ],
      },
    });
    const identity = createCognitoIdentityAdapter({
      client,
      clientId: "seller-client",
      now: () => Date.parse("2026-09-19T11:15:00Z"),
    });
    const authentication = {
      accessToken: "expired-access-token",
      refreshToken: "refresh-token",
      expiresAt: "2026-09-19T11:00:00.000Z",
      sessionExpiresAt: "2026-09-19T18:00:00.000Z",
      principal: {
        subject: "seller-subject",
        email: "owner@example.com",
        name: "Northstar Research",
        sellerId: "sel_123",
        onboardingComplete: true,
      },
    };

    await expect(identity.refresh?.(authentication)).resolves.toMatchObject({
      ok: true,
      value: {
        accessToken: "refreshed-access-token",
        refreshToken: "refresh-token",
        expiresAt: "2026-09-19T12:15:00.000Z",
        sessionExpiresAt: "2026-09-19T18:00:00.000Z",
        principal: { sellerId: "sel_123", onboardingComplete: true },
      },
    });
  });
});
