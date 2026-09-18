import {
  createHmac,
  randomBytes,
  randomUUID,
  scryptSync,
  timingSafeEqual,
} from "node:crypto";
import type {
  AuthResult,
  IdentityAuthentication,
  IdentityChallenge,
  SellerPrincipal,
} from "@/features/auth/model";

const challengeLifetimeMilliseconds = 10 * 60 * 1_000;
const accessLifetimeMilliseconds = 60 * 60 * 1_000;
const passwordSaltBytes = 16;
const passwordHashBytes = 32;
const localTokenPrefix = "apls1";
const minimumSigningSecretBytes = 32;
const maximumSigningSecretBytes = 4096;

type LocalAccount = {
  email: string;
  name: string;
  onboardingComplete: boolean;
  passwordHash: Buffer;
  passwordSalt: Buffer;
  sellerId: string | null;
  storefront: SellerPrincipal["storefront"];
  subject: string;
  verified: boolean;
};

type LocalChallenge = {
  code: string;
  email: string | null;
  expiresAt: number;
  purpose: "recovery" | "verification";
};

type LocalIdentityState = {
  accessTokens: Map<string, IdentityAuthentication>;
  accounts: Map<string, LocalAccount>;
  challenges: Map<string, LocalChallenge>;
};

type LocalIdentityOptions = {
  now?: () => number;
  sellerAccessToken?: string;
  sellerTokenSigningSecret?: string;
};

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

function hashPassword(password: string, salt: Buffer): Buffer {
  return scryptSync(password, salt, passwordHashBytes);
}

function validPassword(password: string): boolean {
  return password.length >= 12 && password.length <= 256;
}

function matchesPassword(account: LocalAccount, password: string): boolean {
  const candidate = hashPassword(password, account.passwordSalt);
  return timingSafeEqual(candidate, account.passwordHash);
}

function createCode(): string {
  return String(randomBytes(4).readUInt32BE(0) % 1_000_000).padStart(6, "0");
}

function principalFromAccount(account: LocalAccount): SellerPrincipal {
  return {
    subject: account.subject,
    email: account.email,
    name: account.name,
    sellerId: account.sellerId,
    storefront: account.storefront,
    onboardingComplete: account.onboardingComplete,
  };
}

function encodeTokenValue(value: string): string {
  return Buffer.from(value, "utf8").toString("base64url");
}

function mintSellerAccessToken(
  account: LocalAccount,
  signingSecret: string,
  issuedAtMilliseconds: number,
): { accessToken: string; expiresAt: string } {
  const issuedAt = Math.floor(issuedAtMilliseconds / 1_000);
  const expiresAt = Math.floor(
    (issuedAtMilliseconds + accessLifetimeMilliseconds) / 1_000,
  );
  const unsignedToken = [
    localTokenPrefix,
    encodeTokenValue(account.subject),
    encodeTokenValue(`local-token:${randomUUID()}`),
    encodeTokenValue(`local-session:${randomUUID()}`),
    String(issuedAt),
    String(expiresAt),
  ].join(".");
  const signature = createHmac("sha256", signingSecret)
    .update(unsignedToken)
    .digest("base64url");
  return {
    accessToken: `${unsignedToken}.${signature}`,
    expiresAt: new Date(expiresAt * 1_000).toISOString(),
  };
}

// createLocalIdentityAdapter provides production-shaped account semantics without AWS.
export function createLocalIdentityAdapter(options: LocalIdentityOptions) {
  const now = options.now ?? Date.now;
  const signingSecret = options.sellerTokenSigningSecret ?? "";
  const signingSecretBytes = Buffer.byteLength(signingSecret, "utf8");
  if (
    signingSecret &&
    (signingSecretBytes < minimumSigningSecretBytes ||
      signingSecretBytes > maximumSigningSecretBytes)
  ) {
    throw new Error(
      "Local identity signing secret must contain 32 to 4096 bytes.",
    );
  }
  if (!signingSecret && !options.sellerAccessToken) {
    throw new Error(
      "Local identity requires a signing secret or compatibility token.",
    );
  }
  const state: LocalIdentityState = {
    accessTokens: new Map(),
    accounts: new Map(),
    challenges: new Map(),
  };

  function createChallenge(
    email: string | null,
    purpose: LocalChallenge["purpose"],
  ): IdentityChallenge & { developmentCode: string } {
    const challengeId = randomUUID();
    const code = createCode();
    state.challenges.set(challengeId, {
      code,
      email,
      expiresAt: now() + challengeLifetimeMilliseconds,
      purpose,
    });
    return { challengeId, developmentCode: code };
  }

  function consumeChallenge(
    challengeId: string,
    code: string,
    purpose: LocalChallenge["purpose"],
  ): AuthResult<LocalChallenge> {
    const challenge = state.challenges.get(challengeId);
    state.challenges.delete(challengeId);
    if (
      !challenge ||
      challenge.purpose !== purpose ||
      challenge.expiresAt <= now() ||
      challenge.code.length !== code.length ||
      !timingSafeEqual(Buffer.from(challenge.code), Buffer.from(code))
    ) {
      return {
        ok: false,
        code: "invalid_code",
        error: "The code is invalid or has expired.",
      };
    }
    return { ok: true, value: challenge };
  }

  return {
    async signUp(input: { email: string; name: string; password: string }) {
      const email = normalizeEmail(input.email);
      const name = input.name.trim();
      if (!email.includes("@") || !name || !validPassword(input.password)) {
        return {
          ok: false as const,
          code: "invalid_request" as const,
          error:
            "Enter a valid name, email, and password of at least 12 characters.",
        };
      }
      if (state.accounts.has(email)) {
        return {
          ok: false as const,
          code: "account_exists" as const,
          error: "An account already exists for this email.",
        };
      }
      const passwordSalt = randomBytes(passwordSaltBytes);
      state.accounts.set(email, {
        email,
        name,
        onboardingComplete: false,
        passwordHash: hashPassword(input.password, passwordSalt),
        passwordSalt,
        sellerId: null,
        storefront: null,
        subject: `local:${randomUUID()}`,
        verified: false,
      });
      return {
        ok: true as const,
        value: createChallenge(email, "verification"),
      };
    },

    async verifyRegistration(input: { challengeId: string; code: string }) {
      const consumed = consumeChallenge(
        input.challengeId,
        input.code.trim(),
        "verification",
      );
      if (!consumed.ok) return consumed;
      const account = consumed.value.email
        ? state.accounts.get(consumed.value.email)
        : undefined;
      if (!account) {
        return {
          ok: false as const,
          code: "invalid_code" as const,
          error: "The code is invalid or has expired.",
        };
      }
      account.verified = true;
      return { ok: true as const, value: undefined };
    },

    async signIn(input: { email: string; password: string }) {
      const account = state.accounts.get(normalizeEmail(input.email));
      if (!account || !matchesPassword(account, input.password)) {
        return {
          ok: false as const,
          code: "invalid_credentials" as const,
          error: "Email or password is incorrect.",
        };
      }
      if (!account.verified) {
        return {
          ok: false as const,
          code: "verification_required" as const,
          error: "Verify your email before signing in.",
        };
      }
      const token = signingSecret
        ? mintSellerAccessToken(account, signingSecret, now())
        : {
            accessToken: options.sellerAccessToken as string,
            expiresAt: new Date(
              now() + accessLifetimeMilliseconds,
            ).toISOString(),
          };
      const authentication: IdentityAuthentication = {
        ...token,
        principal: principalFromAccount(account),
      };
      state.accessTokens.set(authentication.accessToken, authentication);
      return { ok: true as const, value: authentication };
    },

    async beginRecovery(input: { email: string }) {
      const email = normalizeEmail(input.email);
      const account = state.accounts.get(email);
      return {
        ok: true as const,
        value: createChallenge(account ? email : null, "recovery"),
      };
    },

    async completeRecovery(input: {
      challengeId: string;
      code: string;
      password: string;
    }) {
      if (!validPassword(input.password)) {
        return {
          ok: false as const,
          code: "invalid_request" as const,
          error: "Use a password of at least 12 characters.",
        };
      }
      const consumed = consumeChallenge(
        input.challengeId,
        input.code.trim(),
        "recovery",
      );
      if (!consumed.ok) return consumed;
      const account = consumed.value.email
        ? state.accounts.get(consumed.value.email)
        : undefined;
      if (account) {
        const passwordSalt = randomBytes(passwordSaltBytes);
        account.passwordSalt = passwordSalt;
        account.passwordHash = hashPassword(input.password, passwordSalt);
      }
      return { ok: true as const, value: undefined };
    },

    async validate(accessToken: string) {
      const authentication = state.accessTokens.get(accessToken);
      if (!authentication || Date.parse(authentication.expiresAt) <= now()) {
        state.accessTokens.delete(accessToken);
        return {
          ok: false as const,
          code: "session_revoked" as const,
          error: "Your session has ended. Sign in again.",
        };
      }
      return { ok: true as const, value: authentication };
    },

    async revoke(accessToken: string) {
      state.accessTokens.delete(accessToken);
    },

    updatePrincipal(
      subject: string,
      update: Pick<SellerPrincipal, "onboardingComplete" | "sellerId"> &
        Partial<Pick<SellerPrincipal, "storefront">>,
    ) {
      const account = [...state.accounts.values()].find(
        (candidate) => candidate.subject === subject,
      );
      if (!account) return false;
      account.sellerId = update.sellerId;
      account.onboardingComplete = update.onboardingComplete;
      if ("storefront" in update) account.storefront = update.storefront;
      for (const authentication of state.accessTokens.values()) {
        if (authentication.principal.subject === subject) {
          authentication.principal = principalFromAccount(account);
        }
      }
      return true;
    },
  };
}
