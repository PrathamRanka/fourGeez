import { NextResponse } from "next/server";
import { startCommerce } from "@/features/commerce/server/commerce-gateway";

export async function POST(request: Request) {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return NextResponse.json(
      {
        error: {
          code: "invalid_request",
          message: "Invalid checkout request.",
        },
      },
      { status: 400 },
    );
  }
  const result = await startCommerce(input);
  if (!result.ok) {
    return NextResponse.json(
      {
        error: {
          code: result.code,
          message: result.message,
          ...(result.recoveryAction
            ? { details: { recoveryAction: result.recoveryAction } }
            : {}),
        },
      },
      { status: result.status, headers: { "Cache-Control": "no-store" } },
    );
  }
  const response = NextResponse.json(result.value, {
    status: 201,
    headers: { "Cache-Control": "no-store" },
  });
  for (const cookie of result.setCookies ?? []) {
    response.headers.append("Set-Cookie", cookie);
  }
  return response;
}
