import {
  authError,
  authJson,
  pendingRecoveryCookie,
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
  const result = await getIdentityAdapter().beginRecovery({
    email: String(body.email ?? ""),
  });
  if (!result.ok) return authError(result.error, 503);
  await setPendingChallenge(pendingRecoveryCookie, result.value.challengeId);
  return authJson({ developmentCode: result.value.developmentCode });
}
