import { safeRelativeReturnPath } from "@/features/auth/policy";
import {
  authError,
  authJson,
  clearPendingChallenge,
  pendingVerificationCookie,
  readAuthBody,
  readPendingChallenge,
  requireCsrf,
} from "@/features/auth/server/bff";
import { getIdentityAdapter } from "@/features/auth/server/identity";

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const body = await readAuthBody(request, ["code", "returnTo"]);
  const challengeId = await readPendingChallenge(pendingVerificationCookie);
  if (!body || !challengeId) return authError("Start registration again.", 400);
  const result = await getIdentityAdapter().verifyRegistration({
    challengeId,
    code: String(body.code ?? ""),
  });
  if (!result.ok)
    return authError(
      result.error,
      result.code === "dependency_unavailable" ? 503 : 400,
    );
  await clearPendingChallenge(pendingVerificationCookie);
  const returnTo = safeRelativeReturnPath(
    String(body.returnTo ?? "/dashboard"),
  );
  return authJson({
    redirectTo: `/sign-in?verified=1&returnTo=${encodeURIComponent(returnTo)}`,
  });
}
