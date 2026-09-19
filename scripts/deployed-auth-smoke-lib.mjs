import { randomBytes } from "node:crypto";

export const canonicalWebOrigin = "https://agentpay.prathamranka.in";
export const accessTokenRefreshWaitMilliseconds = 60 * 60 * 1_000 - 20_000;
export const absoluteSessionExpiryWaitMilliseconds =
  8 * 60 * 60 * 1_000 + 5_000;

export function generateSmokePassword() {
  return `Ap-${randomBytes(24).toString("base64url")}9!`;
}

export function parseBoolean(value) {
  return /^(1|true|yes)$/i.test(String(value ?? "").trim());
}

export function requireSmokeConfiguration(environment = process.env) {
  const email = environment.AGENTPAY_SMOKE_EMAIL?.trim().toLowerCase();
  if (!email || !email.includes("@")) {
    throw new Error(
      "AGENTPAY_SMOKE_EMAIL must contain the approved private test recipient.",
    );
  }
  const origin = (
    environment.AGENTPAY_SMOKE_ORIGIN ?? canonicalWebOrigin
  ).replace(/\/$/, "");
  if (origin !== canonicalWebOrigin) {
    throw new Error(`Deployed auth smoke is pinned to ${canonicalWebOrigin}.`);
  }
  return {
    email,
    origin,
    waitForExpiry: parseBoolean(environment.AGENTPAY_SMOKE_WAIT_FOR_EXPIRY),
    waitForRefresh: parseBoolean(environment.AGENTPAY_SMOKE_WAIT_FOR_REFRESH),
  };
}

export class CookieJar {
  #cookies = new Map();

  capture(headers) {
    const values = headers.getSetCookie?.() ?? [];
    for (const value of values) {
      const [pair] = value.split(";", 1);
      const separator = pair.indexOf("=");
      if (separator <= 0) continue;
      const name = pair.slice(0, separator).trim();
      const cookieValue = pair.slice(separator + 1).trim();
      if (!cookieValue || /max-age=0/i.test(value)) this.#cookies.delete(name);
      else this.#cookies.set(name, cookieValue);
    }
  }

  header() {
    return [...this.#cookies.entries()]
      .map(([name, value]) => `${name}=${value}`)
      .join("; ");
  }

  value(name) {
    return this.#cookies.get(name);
  }
}

export async function wait(milliseconds) {
  await new Promise((resolve) => setTimeout(resolve, milliseconds));
}
