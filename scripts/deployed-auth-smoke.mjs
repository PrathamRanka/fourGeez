import { createInterface } from "node:readline/promises";
import { stdin, stdout } from "node:process";
import {
  CookieJar,
  absoluteSessionExpiryWaitMilliseconds,
  accessTokenRefreshWaitMilliseconds,
  generateSmokePassword,
  requireSmokeConfiguration,
  wait,
} from "./deployed-auth-smoke-lib.mjs";

const configuration = requireSmokeConfiguration();
const cookies = new CookieJar();
const password = generateSmokePassword();
const name = "AgentPay deployment smoke";

async function request(path, options = {}) {
  const response = await fetch(`${configuration.origin}${path}`, {
    ...options,
    redirect: options.redirect ?? "manual",
    headers: {
      ...(cookies.header() ? { Cookie: cookies.header() } : {}),
      ...options.headers,
    },
  });
  cookies.capture(response.headers);
  return response;
}

async function json(response) {
  const body = await response.text();
  try {
    return body ? JSON.parse(body) : {};
  } catch {
    throw new Error(
      `Expected JSON from ${response.url}; received status ${response.status}.`,
    );
  }
}

function assertStatus(response, expected, step) {
  if (response.status !== expected) {
    throw new Error(
      `${step} returned ${response.status}; expected ${expected}.`,
    );
  }
}

async function csrfToken() {
  const response = await request("/api/auth/csrf");
  assertStatus(response, 200, "CSRF bootstrap");
  const body = await json(response);
  if (!body.csrfToken) throw new Error("CSRF bootstrap returned no token.");
  return body.csrfToken;
}

async function authPost(path, body) {
  const csrf = await csrfToken();
  return request(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Origin: configuration.origin,
      "X-AgentPay-CSRF": csrf,
    },
    body: JSON.stringify(body),
  });
}

async function promptForVerificationCode() {
  const terminal = createInterface({ input: stdin, output: stdout });
  try {
    return (
      await terminal.question(
        "Enter the Cognito verification code from the private test inbox: ",
      )
    ).trim();
  } finally {
    terminal.close();
  }
}

async function assertDashboardRedirect(step) {
  const response = await request("/dashboard", { redirect: "manual" });
  if (response.status < 300 || response.status >= 400) {
    throw new Error(
      `${step} expected an authentication redirect; received ${response.status}.`,
    );
  }
  const location = response.headers.get("location") ?? "";
  if (!location.includes("/sign-in")) {
    throw new Error(`${step} redirected to an unexpected location.`);
  }
}

async function run() {
  stdout.write("Starting canonical deployed seller-auth smoke.\n");
  const signUp = await authPost("/api/auth/sign-up", {
    email: configuration.email,
    name,
    password,
    returnTo: "/dashboard",
  });
  assertStatus(signUp, 200, "Sign-up");
  await json(signUp);
  stdout.write("Sign-up accepted and Cognito verification email requested.\n");

  const code =
    process.env.AGENTPAY_SMOKE_VERIFICATION_CODE?.trim() ||
    (await promptForVerificationCode());
  if (!/^\d{6}$/.test(code))
    throw new Error("Verification code must contain six digits.");
  const verification = await authPost("/api/auth/verify", {
    code,
    returnTo: "/dashboard",
  });
  assertStatus(verification, 200, "Verification");
  await json(verification);
  stdout.write("Email verification accepted.\n");

  const signIn = await authPost("/api/auth/sign-in", {
    email: configuration.email,
    password,
    returnTo: "/dashboard",
  });
  assertStatus(signIn, 200, "Sign-in");
  const signInBody = await json(signIn);
  if (signInBody.redirectTo !== "/dashboard/onboarding") {
    throw new Error(
      `Sign-in redirect was ${signInBody.redirectTo ?? "missing"}; expected /dashboard/onboarding.`,
    );
  }
  if (!cookies.value("__Host-agentpay_seller_session")) {
    throw new Error(
      "Sign-in did not issue the Secure HttpOnly production session cookie.",
    );
  }

  const onboarding = await request("/dashboard/onboarding", {
    redirect: "follow",
  });
  assertStatus(onboarding, 200, "Authenticated onboarding");
  stdout.write("Sign-in, sealed session, and onboarding redirect passed.\n");

  if (configuration.waitForRefresh) {
    stdout.write(
      "Waiting for the deployed Cognito access-token refresh boundary.\n",
    );
    const beforeRefresh = cookies.value("__Host-agentpay_seller_session");
    await wait(accessTokenRefreshWaitMilliseconds);
    const refreshed = await request("/dashboard", { redirect: "follow" });
    assertStatus(refreshed, 200, "Session refresh");
    if (cookies.value("__Host-agentpay_seller_session") === beforeRefresh) {
      throw new Error(
        "The production session cookie was not rotated at refresh.",
      );
    }
    stdout.write("Session refresh passed.\n");
  }

  if (configuration.waitForExpiry) {
    stdout.write(
      "Waiting for the eight-hour absolute seller-session boundary.\n",
    );
    await wait(absoluteSessionExpiryWaitMilliseconds);
    await assertDashboardRedirect("Absolute session expiry");
    stdout.write("Absolute session expiry passed.\n");
    const resumedSignIn = await authPost("/api/auth/sign-in", {
      email: configuration.email,
      password,
      returnTo: "/dashboard",
    });
    assertStatus(resumedSignIn, 200, "Sign-in after absolute expiry");
    await json(resumedSignIn);
  }

  const signOut = await authPost("/api/auth/sign-out", {});
  assertStatus(signOut, 200, "Sign-out");
  await assertDashboardRedirect("Post-sign-out dashboard access");
  stdout.write("Sign-out and post-authentication rejection passed.\n");
}

run().catch((error) => {
  process.exitCode = 1;
  stdout.write(
    `Deployed auth smoke failed: ${error instanceof Error ? error.message : String(error)}\n`,
  );
});
