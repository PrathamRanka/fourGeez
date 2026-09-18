"use client";

import { LogOut } from "lucide-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";

export function SignOutButton() {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function signOut() {
    setPending(true);
    setError(null);
    try {
      const csrfResponse = await fetch("/api/auth/csrf", {
        credentials: "same-origin",
      });
      const csrfBody = (await csrfResponse.json()) as { csrfToken?: string };
      if (!csrfResponse.ok || !csrfBody.csrfToken) throw new Error();
      const response = await fetch("/api/auth/sign-out", {
        method: "POST",
        credentials: "same-origin",
        headers: { "X-AgentPay-CSRF": csrfBody.csrfToken },
      });
      if (!response.ok) throw new Error();
      router.push("/sign-in");
      router.refresh();
    } catch {
      setError("Sign-out failed. Refresh the page and try again.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="dashboard-sign-out">
      <Button
        type="button"
        variant="ghost"
        onClick={signOut}
        disabled={pending}
      >
        <LogOut aria-hidden="true" />
        {pending ? "Signing out…" : "Sign out"}
      </Button>
      {error ? <span role="alert">{error}</span> : null}
    </div>
  );
}
