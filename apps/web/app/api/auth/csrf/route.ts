import { authJson, issueCsrfToken } from "@/features/auth/server/bff";

export async function GET() {
  return authJson({ csrfToken: await issueCsrfToken() });
}
