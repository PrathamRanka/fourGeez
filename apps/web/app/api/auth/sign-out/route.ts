import { authError, authJson, requireCsrf } from "@/features/auth/server/bff";
import { destroySellerSession } from "@/features/auth/server/session";

export async function POST(request: Request) {
  const csrf = await requireCsrf(request);
  if (!csrf.ok) return authError(csrf.error, 403);
  const revocationConfirmed = await destroySellerSession();
  if (!revocationConfirmed) {
    return authError(
      "The local session ended, but server revocation could not be confirmed.",
      503,
    );
  }
  return authJson({ redirectTo: "/sign-in" });
}
