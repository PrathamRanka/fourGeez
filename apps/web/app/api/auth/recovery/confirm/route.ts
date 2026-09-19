import {
  authError,
  authJson,
  clearPendingChallenge,
  pendingRecoveryCookie,
  readAuthBody,
  readPendingChallenge,
  requireCsrf,
} from "@/features/auth/server/bff";
import { getIdentityAdapter } from "@/features/auth/server/identity";

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const body = await readAuthBody(request, ["code", "password"]);
  const challengeId = await readPendingChallenge(pendingRecoveryCookie);
  if (!body || !challengeId)
    return authError("Request a new recovery code.", 400);
  const result = await getIdentityAdapter().completeRecovery({
    challengeId,
    code: String(body.code ?? ""),
    password: String(body.password ?? ""),
  });
  if (!result.ok)
    return authError(
      result.error,
      result.code === "dependency_unavailable" ? 503 : 400,
    );
  await clearPendingChallenge(pendingRecoveryCookie);
  return authJson({ redirectTo: "/sign-in?recovered=1" });
}
