import { NextResponse } from "next/server";
import { completeCommerce } from "@/features/commerce/server/commerce-gateway";

export async function POST(request: Request) {
  let input: unknown;
  try {
    input = await request.json();
  } catch {
    return NextResponse.json(
      {
        error: { code: "invalid_request", message: "Invalid payment request." },
      },
      { status: 400 },
    );
  }
  const result = await completeCommerce(
    input,
    request.headers.get("cookie") ?? "",
  );
  return result.ok
    ? NextResponse.json(result.value, {
        status: 200,
        headers: { "Cache-Control": "no-store" },
      })
    : NextResponse.json(
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
