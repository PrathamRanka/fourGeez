import { NextResponse, type NextRequest } from "next/server";
import { safeRelativeReturnPath } from "@/features/auth/policy";

export function proxy(request: NextRequest) {
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set(
    "x-agentpay-return-path",
    safeRelativeReturnPath(
      `${request.nextUrl.pathname}${request.nextUrl.search}`,
    ),
  );
  return NextResponse.next({ request: { headers: requestHeaders } });
}

export const config = { matcher: ["/dashboard/:path*"] };
