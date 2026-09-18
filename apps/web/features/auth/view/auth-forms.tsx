"use client";

import { LoaderCircle, LogIn, MailCheck, ShieldCheck } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import styles from "./auth-surface.module.css";

type AuthResponse = {
  developmentCode?: string;
  error?: string;
  redirectTo?: string;
};

function useSecureAuthForm() {
  const router = useRouter();
  const [csrfToken, setCsrfToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [developmentCode, setDevelopmentCode] = useState<string | null>(null);
  const [nextPath, setNextPath] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    fetch("/api/auth/csrf", { credentials: "same-origin" })
      .then(async (response) => {
        const body = (await response.json()) as { csrfToken?: string };
        if (!response.ok || !body.csrfToken) throw new Error();
        if (active) setCsrfToken(body.csrfToken);
      })
      .catch(() => {
        if (active)
          setError("Secure form setup failed. Refresh and try again.");
      });
    return () => {
      active = false;
    };
  }, []);

  async function submit(endpoint: string, payload: Record<string, string>) {
    if (!csrfToken) {
      setError("Secure form setup is still loading. Try again in a moment.");
      return;
    }
    setPending(true);
    setError(null);
    try {
      const response = await fetch(endpoint, {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          "X-AgentPay-CSRF": csrfToken,
        },
        body: JSON.stringify(payload),
      });
      const body = (await response.json()) as AuthResponse;
      if (!response.ok) {
        setError(body.error ?? "AgentPay could not complete this request.");
        return;
      }
      setDevelopmentCode(body.developmentCode ?? null);
      setNextPath(body.redirectTo ?? null);
      if (body.redirectTo && !body.developmentCode) {
        router.push(body.redirectTo);
        router.refresh();
      }
    } catch {
      setError(
        "Authentication is unavailable. Check the connection and retry.",
      );
    } finally {
      setPending(false);
    }
  }

  return {
    csrfReady: csrfToken !== null,
    developmentCode,
    error,
    pending,
    nextPath,
    submit,
  };
}

function FormMessage({ error }: { error: string | null }) {
  return error ? (
    <p className={styles.message} role="alert">
      {error}
    </p>
  ) : null;
}

function DevelopmentCode({
  code,
  nextPath,
}: {
  code: string | null;
  nextPath: string | null;
}) {
  const router = useRouter();
  return code ? (
    <div className={styles.developmentCode} role="status">
      <strong>Local development code</strong>
      <code>{code}</code>
      {nextPath ? (
        <Button
          className={styles.secondaryAction}
          type="button"
          variant="outline"
          onClick={() => router.push(nextPath)}
        >
          Continue to verification
        </Button>
      ) : null}
    </div>
  ) : null;
}

function FormHeading({
  label,
  title,
  description,
}: {
  label: string;
  title: string;
  description: string;
}) {
  return (
    <header className={styles.formHeading}>
      <span>{label}</span>
      <h2>{title}</h2>
      <p>{description}</p>
    </header>
  );
}

export function SignUpForm({ returnTo }: { returnTo: string }) {
  const form = useSecureAuthForm();
  return (
    <div className={styles.formPanel}>
      <FormHeading
        label="Seller workspace preview"
        title="Create your seller account"
        description="Verify your email before connecting a storefront or payment destination."
      />
      <form
        className={styles.form}
        aria-label="Create seller account"
        onSubmit={(event) => {
          event.preventDefault();
          const fields = new FormData(event.currentTarget);
          void form.submit("/api/auth/sign-up", {
            name: String(fields.get("name") ?? ""),
            email: String(fields.get("email") ?? ""),
            password: String(fields.get("password") ?? ""),
            returnTo,
          });
        }}
      >
        <label>
          <span>Your name</span>
          <input name="name" autoComplete="name" required maxLength={120} />
        </label>
        <label>
          <span>Work email</span>
          <input name="email" type="email" autoComplete="email" required />
        </label>
        <label>
          <span>Password</span>
          <input
            name="password"
            type="password"
            autoComplete="new-password"
            minLength={12}
            maxLength={256}
            required
          />
          <small>Use at least 12 characters.</small>
        </label>
        <FormMessage error={form.error} />
        <DevelopmentCode code={form.developmentCode} nextPath={form.nextPath} />
        <Button
          className={styles.submit}
          type="submit"
          size="lg"
          disabled={!form.csrfReady || form.pending}
        >
          {form.pending ? (
            <LoaderCircle className="animate-spin" aria-hidden="true" />
          ) : (
            <ShieldCheck aria-hidden="true" />
          )}
          {form.pending ? "Creating account…" : "Create seller account"}
        </Button>
      </form>
    </div>
  );
}

export function VerifyForm({ returnTo }: { returnTo: string }) {
  const form = useSecureAuthForm();
  return (
    <div className={styles.formPanel}>
      <FormHeading
        label="Identity checkpoint"
        title="Verify your email"
        description="Enter the one-time code sent by your identity provider."
      />
      <form
        className={styles.form}
        aria-label="Verify seller email"
        onSubmit={(event) => {
          event.preventDefault();
          const fields = new FormData(event.currentTarget);
          void form.submit("/api/auth/verify", {
            code: String(fields.get("code") ?? ""),
            returnTo,
          });
        }}
      >
        <label>
          <span>Verification code</span>
          <input
            name="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]{6}"
            required
          />
        </label>
        <FormMessage error={form.error} />
        <Button
          className={styles.submit}
          type="submit"
          size="lg"
          disabled={!form.csrfReady || form.pending}
        >
          {form.pending ? (
            <LoaderCircle className="animate-spin" aria-hidden="true" />
          ) : (
            <MailCheck aria-hidden="true" />
          )}
          {form.pending ? "Verifying…" : "Verify email"}
        </Button>
      </form>
    </div>
  );
}

export function SignInForm({ returnTo }: { returnTo: string }) {
  const form = useSecureAuthForm();
  return (
    <div className={styles.formPanel}>
      <FormHeading
        label="Protected seller access"
        title="Sign in to your storefront"
        description="Your session stays in a secure same-origin cookie and is never stored in browser storage."
      />
      <form
        className={styles.form}
        aria-label="Seller sign in"
        onSubmit={(event) => {
          event.preventDefault();
          const fields = new FormData(event.currentTarget);
          void form.submit("/api/auth/sign-in", {
            email: String(fields.get("email") ?? ""),
            password: String(fields.get("password") ?? ""),
            returnTo,
          });
        }}
      >
        <label>
          <span>Work email</span>
          <input name="email" type="email" autoComplete="email" required />
        </label>
        <label>
          <span>Password</span>
          <input
            name="password"
            type="password"
            autoComplete="current-password"
            required
          />
        </label>
        <FormMessage error={form.error} />
        <Button
          className={styles.submit}
          type="submit"
          size="lg"
          disabled={!form.csrfReady || form.pending}
        >
          {form.pending ? (
            <LoaderCircle className="animate-spin" aria-hidden="true" />
          ) : (
            <LogIn aria-hidden="true" />
          )}
          {form.pending ? "Signing in…" : "Sign in"}
        </Button>
      </form>
    </div>
  );
}

export function RecoveryForms() {
  const form = useSecureAuthForm();
  return (
    <div className={styles.recoveryGrid}>
      <section className={styles.formPanel}>
        <FormHeading
          label="Two-step recovery"
          title="Request a recovery code"
          description="The response is the same whether or not the account exists."
        />
        <form
          className={styles.form}
          aria-label="Request password recovery"
          onSubmit={(event) => {
            event.preventDefault();
            const fields = new FormData(event.currentTarget);
            void form.submit("/api/auth/recovery/request", {
              email: String(fields.get("email") ?? ""),
            });
          }}
        >
          <label>
            <span>Work email</span>
            <input name="email" type="email" autoComplete="email" required />
          </label>
          <Button
            className={styles.submit}
            type="submit"
            size="lg"
            disabled={!form.csrfReady || form.pending}
          >
            Send recovery code
          </Button>
        </form>
      </section>
      <section className={styles.formPanel}>
        <FormHeading
          label="Confirm ownership"
          title="Choose a new password"
          description="Recovery creates no browser session; sign in again after it succeeds."
        />
        <form
          className={styles.form}
          aria-label="Complete password recovery"
          onSubmit={(event) => {
            event.preventDefault();
            const fields = new FormData(event.currentTarget);
            void form.submit("/api/auth/recovery/confirm", {
              code: String(fields.get("code") ?? ""),
              password: String(fields.get("password") ?? ""),
            });
          }}
        >
          <label>
            <span>Recovery code</span>
            <input
              name="code"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              required
            />
          </label>
          <label>
            <span>New password</span>
            <input
              name="password"
              type="password"
              autoComplete="new-password"
              minLength={12}
              maxLength={256}
              required
            />
          </label>
          <FormMessage error={form.error} />
          <DevelopmentCode
            code={form.developmentCode}
            nextPath={form.nextPath}
          />
          <Button
            className={styles.submit}
            type="submit"
            size="lg"
            disabled={!form.csrfReady || form.pending}
          >
            Save new password
          </Button>
        </form>
      </section>
    </div>
  );
}
