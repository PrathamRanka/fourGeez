import { timingSafeEqual } from "node:crypto";

type CsrfRequest = {
  allowedOrigin: string;
  cookieToken?: string | null;
  headerToken?: string | null;
  origin?: string | null;
};

export type CsrfValidation =
  | { ok: true }
  | { ok: false; error: "Invalid request origin." | "Invalid security token." };

function equalTokens(left: string, right: string): boolean {
  const leftBytes = Buffer.from(left);
  const rightBytes = Buffer.from(right);
  return (
    leftBytes.length === rightBytes.length &&
    timingSafeEqual(leftBytes, rightBytes)
  );
}

// validateCsrfRequest requires exact Origin and double-submit token matching.
export function validateCsrfRequest(input: CsrfRequest): CsrfValidation {
  if (input.origin !== input.allowedOrigin) {
    return { ok: false, error: "Invalid request origin." };
  }
  if (
    !input.cookieToken ||
    !input.headerToken ||
    !equalTokens(input.cookieToken, input.headerToken)
  ) {
    return { ok: false, error: "Invalid security token." };
  }
  return { ok: true };
}
