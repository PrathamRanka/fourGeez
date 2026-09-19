export type SellerPrincipal = {
  subject: string;
  email: string;
  name: string;
  sellerId: string | null;
  onboardingComplete: boolean;
  storefront?: {
    sellerId: string;
    name: string;
    slug: string;
    upstreamBaseUrl: string;
    status: "active" | "draft" | "suspended";
    version: number;
  } | null;
};

export type AuthFailureCode =
  | "account_exists"
  | "dependency_unavailable"
  | "invalid_code"
  | "invalid_credentials"
  | "invalid_request"
  | "session_revoked"
  | "verification_required";

export type AuthResult<Value> =
  | { ok: true; value: Value }
  | { ok: false; code: AuthFailureCode; error: string };

export type IdentityAuthentication = {
  accessToken: string;
  expiresAt: string;
  principal: SellerPrincipal;
  refreshToken?: string;
  sessionExpiresAt?: string;
};

export type IdentityChallenge = {
  challengeId: string;
  developmentCode?: string;
};

export type IdentityAdapter = {
  signUp(input: {
    email: string;
    name: string;
    password: string;
  }): Promise<AuthResult<IdentityChallenge>>;
  verifyRegistration(input: {
    challengeId: string;
    code: string;
  }): Promise<AuthResult<void>>;
  resendVerification?(input: {
    email: string;
  }): Promise<AuthResult<IdentityChallenge>>;
  signIn(input: {
    email: string;
    password: string;
  }): Promise<AuthResult<IdentityAuthentication>>;
  beginRecovery(input: {
    email: string;
  }): Promise<AuthResult<IdentityChallenge>>;
  completeRecovery(input: {
    challengeId: string;
    code: string;
    password: string;
  }): Promise<AuthResult<void>>;
  validate(
    authentication: IdentityAuthentication | string,
  ): Promise<AuthResult<IdentityAuthentication>>;
  refresh?(
    authentication: IdentityAuthentication,
  ): Promise<AuthResult<IdentityAuthentication>>;
  revoke(authentication: IdentityAuthentication | string): Promise<void>;
};
