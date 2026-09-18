import { postAuthenticationPath } from "@/features/auth/policy";
import {
  authError,
  authJson,
  readAuthBody,
  requireCsrf,
} from "@/features/auth/server/bff";
import { getIdentityAdapter } from "@/features/auth/server/identity";
import { establishSellerSession } from "@/features/auth/server/session";

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const body = await readAuthBody(request, ["email", "password", "returnTo"]);
  if (!body) return authError("Check your sign-in details and try again.", 400);
  const result = await getIdentityAdapter().signIn({
    email: String(body.email ?? ""),
    password: String(body.password ?? ""),
  });
  if (!result.ok)
    return authError(
      result.error,
      result.code === "dependency_unavailable" ? 503 : 401,
    );
  await establishSellerSession(result.value);
  return authJson({
    redirectTo: postAuthenticationPath(
      result.value.principal,
      String(body.returnTo ?? "/dashboard"),
    ),
  });
}
