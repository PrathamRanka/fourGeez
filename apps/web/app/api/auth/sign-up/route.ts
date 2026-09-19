import { getIdentityAdapter } from "@/features/auth/server/identity";
import {
  authError,
  authJson,
  pendingVerificationCookie,
  readAuthBody,
  requireCsrf,
  setPendingChallenge,
} from "@/features/auth/server/bff";
import { safeRelativeReturnPath } from "@/features/auth/policy";

const allowedFields = ["email", "name", "password", "returnTo"] as const;

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const body = await readAuthBody(request, allowedFields);
  if (!body)
    return authError("Check the registration details and try again.", 400);
  const result = await getIdentityAdapter().signUp({
    email: String(body.email ?? ""),
    name: String(body.name ?? ""),
    password: String(body.password ?? ""),
  });
  if (!result.ok)
    return authError(
      result.error,
      result.code === "dependency_unavailable"
        ? 503
        : result.code === "account_exists"
          ? 409
          : 400,
    );
  await setPendingChallenge(
    pendingVerificationCookie,
    result.value.challengeId,
  );
  const returnTo = safeRelativeReturnPath(
    String(body.returnTo ?? "/dashboard"),
  );
  return authJson({
    developmentCode: result.value.developmentCode,
    redirectTo: `/verify?returnTo=${encodeURIComponent(returnTo)}`,
  });
}
