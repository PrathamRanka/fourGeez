const defaultDashboardPath = "/dashboard";
const onboardingPath = "/dashboard/onboarding";
const maximumReturnPathLength = 2_048;

type AccountRedirectState = {
  onboardingComplete: boolean;
  sellerId: string | null;
};

// safeRelativeReturnPath accepts only same-origin application paths.
export function safeRelativeReturnPath(candidate?: string | null): string {
  if (
    !candidate ||
    candidate.length > maximumReturnPathLength ||
    !candidate.startsWith("/") ||
    candidate.startsWith("//") ||
    candidate.includes("\\") ||
    candidate.includes("#") ||
    /%(?:2f|5c)/i.test(candidate) ||
    /[\u0000-\u001f\u007f]/.test(candidate)
  ) {
    return defaultDashboardPath;
  }

  try {
    const parsed = new URL(candidate, "https://agentpay.invalid");
    if (parsed.origin !== "https://agentpay.invalid") {
      return defaultDashboardPath;
    }
    return `${parsed.pathname}${parsed.search}`;
  } catch {
    return defaultDashboardPath;
  }
}

// postAuthenticationPath keeps incomplete sellers in the resumable setup journey.
export function postAuthenticationPath(
  account: AccountRedirectState,
  requestedPath?: string | null,
): string {
  if (!account.sellerId || !account.onboardingComplete) {
    return onboardingPath;
  }
  return safeRelativeReturnPath(requestedPath);
}
