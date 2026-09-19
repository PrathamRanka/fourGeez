import { NextResponse, type NextRequest } from "next/server";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import {
  refreshSellerSessionValue,
  sellerSessionCookieName,
} from "@/features/auth/server/session";

export async function proxy(request: NextRequest) {
  const requestHeaders = new Headers(request.headers);
  const returnPath = safeRelativeReturnPath(
    `${request.nextUrl.pathname}${request.nextUrl.search}`,
  );
  requestHeaders.set(
    "x-agentpay-return-path",
    returnPath,
  );
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  const cookieName = sellerSessionCookieName();
  const cookieValue = request.cookies.get(cookieName)?.value;
  if (!cookieValue) return response;
  const refresh = await refreshSellerSessionValue(cookieValue);
  if (refresh.status === "current") return response;
  if (refresh.status === "expired") {
    const redirect = NextResponse.redirect(
      new URL(
        `/sign-in?returnTo=${encodeURIComponent(returnPath)}&expired=1`,
        request.url,
      ),
    );
    redirect.cookies.delete(cookieName);
    return redirect;
  }
  response.cookies.set(cookieName, refresh.cookieValue, {
    httpOnly: true,
    maxAge: Math.max(
      0,
      Math.floor((Date.parse(refresh.expiresAt) - Date.now()) / 1_000),
    ),
    path: "/",
    sameSite: "strict",
    secure: true,
  });
  return response;
}

export const config = { matcher: ["/dashboard/:path*"] };
