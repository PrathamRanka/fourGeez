import {
  authError,
  authJson,
  pendingVerificationCookie,
  readAuthBody,
  requireCsrf,
  setPendingChallenge,
} from "@/features/auth/server/bff";
import { getIdentityAdapter } from "@/features/auth/server/identity";

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const body = await readAuthBody(request, ["email"]);
  if (!body) return authError("Enter a valid email address.", 400);
  const resendVerification = getIdentityAdapter().resendVerification;
  if (!resendVerification) {
    return authError("Seller authentication is temporarily unavailable.", 503);
  }
  const result = await resendVerification({
    email: String(body.email ?? ""),
  });
  if (!result.ok) {
    return authError(
      result.error,
      result.code === "dependency_unavailable" ? 503 : 400,
    );
  }
  await setPendingChallenge(
    pendingVerificationCookie,
    result.value.challengeId,
  );
  return authJson({ developmentCode: result.value.developmentCode });
}
