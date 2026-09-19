import {
  CognitoIdentityProviderClient,
  ConfirmForgotPasswordCommand,
  ConfirmSignUpCommand,
  ForgotPasswordCommand,
  GetUserCommand,
  GlobalSignOutCommand,
  InitiateAuthCommand,
  ResendConfirmationCodeCommand,
  SignUpCommand,
  type AttributeType,
} from "@aws-sdk/client-cognito-identity-provider";
import type {
  AuthResult,
  IdentityAdapter,
  IdentityAuthentication,
  SellerPrincipal,
} from "@/features/auth/model";

const minimumPasswordLength = 12;
const maximumPasswordLength = 256;
const sellerSessionLifetimeMilliseconds = 8 * 60 * 60 * 1_000;

type CognitoCommandClient = Pick<CognitoIdentityProviderClient, "send">;

type CognitoIdentityOptions = {
  client?: CognitoCommandClient;
  clientId: string;
  now?: () => number;
  region?: string;
};

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

function validPassword(password: string): boolean {
  return (
    password.length >= minimumPasswordLength &&
    password.length <= maximumPasswordLength
  );
}

function errorName(error: unknown): string {
  return error instanceof Error ? error.name : "";
}

function invalidCredentials(): AuthResult<never> {
  return {
    ok: false,
    code: "invalid_credentials",
    error: "Email or password is incorrect.",
  };
}

function dependencyUnavailable(): AuthResult<never> {
  return {
    ok: false,
    code: "dependency_unavailable",
    error: "Seller authentication is temporarily unavailable.",
  };
}

function attributeValue(
  attributes: AttributeType[] | undefined,
  name: string,
): string {
  return attributes?.find((attribute) => attribute.Name === name)?.Value ?? "";
}

function principalFromAttributes(
  attributes: AttributeType[] | undefined,
  username: string | undefined,
  current?: SellerPrincipal,
): SellerPrincipal | null {
  const subject = attributeValue(attributes, "sub");
  const email = normalizeEmail(attributeValue(attributes, "email"));
  if (!subject || !email) return null;
  return {
    subject,
    email,
    name: attributeValue(attributes, "name").trim() || email || username || "",
    sellerId: current?.sellerId ?? null,
    onboardingComplete: current?.onboardingComplete ?? false,
    storefront: current?.storefront,
  };
}

export function createCognitoIdentityAdapter(
  options: CognitoIdentityOptions,
): IdentityAdapter {
  const clientId = options.clientId.trim();
  const region = options.region?.trim();
  if (!clientId) throw new Error("Cognito app client ID is required.");
  const client =
    options.client ??
    new CognitoIdentityProviderClient({
      region,
    });
  const now = options.now ?? Date.now;

  async function currentPrincipal(
    accessToken: string,
    current?: SellerPrincipal,
  ): Promise<AuthResult<SellerPrincipal>> {
    try {
      const response = await client.send(
        new GetUserCommand({ AccessToken: accessToken }),
      );
      const principal = principalFromAttributes(
        response.UserAttributes,
        response.Username,
        current,
      );
      return principal
        ? { ok: true, value: principal }
        : {
            ok: false,
            code: "invalid_credentials",
            error: "Seller identity claims are incomplete.",
          };
    } catch (error) {
      if (
        errorName(error) === "NotAuthorizedException" ||
        errorName(error) === "UserNotFoundException"
      ) {
        return {
          ok: false,
          code: "session_revoked",
          error: "Your session has ended. Sign in again.",
        };
      }
      return dependencyUnavailable();
    }
  }

  return {
    async signUp(input) {
      const email = normalizeEmail(input.email);
      const name = input.name.trim();
      if (!email.includes("@") || !name || !validPassword(input.password)) {
        return {
          ok: false,
          code: "invalid_request",
          error:
            "Enter a valid name, email, and password of at least 12 characters.",
        };
      }
      try {
        await client.send(
          new SignUpCommand({
            ClientId: clientId,
            Username: email,
            Password: input.password,
            UserAttributes: [
              { Name: "email", Value: email },
              { Name: "name", Value: name },
            ],
          }),
        );
        return { ok: true, value: { challengeId: email } };
      } catch (error) {
        if (errorName(error) === "UsernameExistsException") {
          return {
            ok: false,
            code: "account_exists",
            error: "An account already exists for this email.",
          };
        }
        if (
          errorName(error) === "InvalidPasswordException" ||
          errorName(error) === "InvalidParameterException"
        ) {
          return {
            ok: false,
            code: "invalid_request",
            error: "Check the registration details and try again.",
          };
        }
        return dependencyUnavailable();
      }
    },

    async verifyRegistration(input) {
      try {
        await client.send(
          new ConfirmSignUpCommand({
            ClientId: clientId,
            Username: normalizeEmail(input.challengeId),
            ConfirmationCode: input.code.trim(),
          }),
        );
        return { ok: true, value: undefined };
      } catch (error) {
        if (
          errorName(error) === "CodeMismatchException" ||
          errorName(error) === "ExpiredCodeException" ||
          errorName(error) === "UserNotFoundException"
        ) {
          return {
            ok: false,
            code: "invalid_code",
            error: "The code is invalid or has expired.",
          };
        }
        return dependencyUnavailable();
      }
    },

    async resendVerification(input) {
      const email = normalizeEmail(input.email);
      if (!email.includes("@")) {
        return {
          ok: false,
          code: "invalid_request",
          error: "Enter a valid email address.",
        };
      }
      try {
        await client.send(
          new ResendConfirmationCodeCommand({
            ClientId: clientId,
            Username: email,
          }),
        );
        return { ok: true, value: { challengeId: email } };
      } catch (error) {
        if (
          errorName(error) === "UserNotFoundException" ||
          errorName(error) === "InvalidParameterException"
        ) {
          return {
            ok: false,
            code: "invalid_request",
            error: "This account cannot receive a new verification code.",
          };
        }
        return dependencyUnavailable();
      }
    },

    async signIn(input) {
      const email = normalizeEmail(input.email);
      if (!email.includes("@") || !input.password) return invalidCredentials();
      try {
        const response = await client.send(
          new InitiateAuthCommand({
            ClientId: clientId,
            AuthFlow: "USER_PASSWORD_AUTH",
            AuthParameters: {
              USERNAME: email,
              PASSWORD: input.password,
            },
          }),
        );
        const authentication = response.AuthenticationResult;
        if (!authentication?.AccessToken || !authentication.ExpiresIn) {
          return dependencyUnavailable();
        }
        const principal = await currentPrincipal(authentication.AccessToken);
        if (!principal.ok) return principal;
        return {
          ok: true,
          value: {
            accessToken: authentication.AccessToken,
            refreshToken: authentication.RefreshToken,
            expiresAt: new Date(
              now() + authentication.ExpiresIn * 1_000,
            ).toISOString(),
            sessionExpiresAt: new Date(
              now() + sellerSessionLifetimeMilliseconds,
            ).toISOString(),
            principal: principal.value,
          },
        };
      } catch (error) {
        if (errorName(error) === "UserNotConfirmedException") {
          return {
            ok: false,
            code: "verification_required",
            error: "Verify your email before signing in.",
          };
        }
        if (
          errorName(error) === "NotAuthorizedException" ||
          errorName(error) === "UserNotFoundException"
        ) {
          return invalidCredentials();
        }
        return dependencyUnavailable();
      }
    },

    async beginRecovery(input) {
      const email = normalizeEmail(input.email);
      if (!email.includes("@")) {
        return {
          ok: false,
          code: "invalid_request",
          error: "Enter a valid email address.",
        };
      }
      try {
        await client.send(
          new ForgotPasswordCommand({ ClientId: clientId, Username: email }),
        );
      } catch (error) {
        if (errorName(error) !== "UserNotFoundException") {
          return dependencyUnavailable();
        }
      }
      return { ok: true, value: { challengeId: email } };
    },

    async completeRecovery(input) {
      if (!validPassword(input.password)) {
        return {
          ok: false,
          code: "invalid_request",
          error: "Use a password of at least 12 characters.",
        };
      }
      try {
        await client.send(
          new ConfirmForgotPasswordCommand({
            ClientId: clientId,
            Username: normalizeEmail(input.challengeId),
            ConfirmationCode: input.code.trim(),
            Password: input.password,
          }),
        );
        return { ok: true, value: undefined };
      } catch (error) {
        if (
          errorName(error) === "CodeMismatchException" ||
          errorName(error) === "ExpiredCodeException" ||
          errorName(error) === "UserNotFoundException"
        ) {
          return {
            ok: false,
            code: "invalid_code",
            error: "The code is invalid or has expired.",
          };
        }
        if (errorName(error) === "InvalidPasswordException") {
          return {
            ok: false,
            code: "invalid_request",
            error: "Use a password that meets the account security policy.",
          };
        }
        return dependencyUnavailable();
      }
    },

    async validate(candidate: IdentityAuthentication | string) {
      if (typeof candidate === "string") {
        return {
          ok: false,
          code: "session_revoked",
          error: "Your session has ended. Sign in again.",
        };
      }
      const authentication = candidate;
      if (Date.parse(authentication.expiresAt) <= now()) {
        return {
          ok: false,
          code: "session_revoked",
          error: "Your session has ended. Sign in again.",
        };
      }
      const principal = await currentPrincipal(
        authentication.accessToken,
        authentication.principal,
      );
      return principal.ok
        ? {
            ok: true,
            value: { ...authentication, principal: principal.value },
          }
        : principal;
    },

    async refresh(authentication: IdentityAuthentication) {
      if (
        !authentication.refreshToken ||
        Date.parse(
          authentication.sessionExpiresAt ?? authentication.expiresAt,
        ) <= now()
      ) {
        return {
          ok: false,
          code: "session_revoked",
          error: "Your session has ended. Sign in again.",
        };
      }
      try {
        const response = await client.send(
          new InitiateAuthCommand({
            ClientId: clientId,
            AuthFlow: "REFRESH_TOKEN_AUTH",
            AuthParameters: {
              REFRESH_TOKEN: authentication.refreshToken,
            },
          }),
        );
        const refreshed = response.AuthenticationResult;
        if (!refreshed?.AccessToken || !refreshed.ExpiresIn) {
          return dependencyUnavailable();
        }
        const principal = await currentPrincipal(
          refreshed.AccessToken,
          authentication.principal,
        );
        if (!principal.ok) return principal;
        return {
          ok: true,
          value: {
            ...authentication,
            accessToken: refreshed.AccessToken,
            expiresAt: new Date(
              now() + refreshed.ExpiresIn * 1_000,
            ).toISOString(),
            principal: principal.value,
          },
        };
      } catch (error) {
        if (
          errorName(error) === "NotAuthorizedException" ||
          errorName(error) === "UserNotFoundException"
        ) {
          return {
            ok: false,
            code: "session_revoked",
            error: "Your session has ended. Sign in again.",
          };
        }
        return dependencyUnavailable();
      }
    },

    async revoke(candidate: IdentityAuthentication | string) {
      if (typeof candidate === "string") return;
      try {
        await client.send(
          new GlobalSignOutCommand({
            AccessToken: candidate.accessToken,
          }),
        );
      } catch {
        // The BFF always removes its encrypted cookie. The Go API owns the
        // authoritative token-family revocation used by seller routes.
      }
    },
  };
}
