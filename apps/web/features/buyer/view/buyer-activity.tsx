"use client";

import { Bot, CheckCircle2, LoaderCircle, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import type {
  BuyerActivityAction,
  BuyerActivityResult,
} from "@/features/buyer/model";

export function BuyerActivity({ run }: { run: BuyerActivityAction }) {
  const [result, setResult] = useState<BuyerActivityResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    const fields = new FormData(event.currentTarget);
    const response = await run({
      slug: String(fields.get("slug") ?? ""),
      prompt: String(fields.get("prompt") ?? ""),
    });
    setPending(false);
    if (!response.ok) {
      setError(response.error);
      return;
    }
    setResult(response.value);
  }

  return (
    <main id="main-content" className="buyer-activity-page">
      <header>
        <p>Buyer workspace</p>
        <h1>Ask AgentPay to inspect a storefront.</h1>
        <span>See each bounded tool step before any purchase action is taken.</span>
      </header>
      <div className="buyer-activity-layout">
        <form aria-label="Buyer request" onSubmit={submit}>
          <label><span>Storefront slug</span><input name="slug" required placeholder="northstar" /></label>
          <label><span>What do you need?</span><textarea name="prompt" required minLength={3} maxLength={500} placeholder="A market brief under 40 USDC" /></label>
          <Button type="submit" disabled={pending}>
            {pending ? <LoaderCircle className="animate-spin" aria-hidden="true" /> : <Bot aria-hidden="true" />}
            Inspect storefront
          </Button>
          {error ? <p role="alert">{error}</p> : null}
        </form>
        <section aria-label="Agent response">
          <div className="buyer-mode"><ShieldCheck aria-hidden="true" /><span>Deterministic fallback active</span></div>
          {result ? (
            <>
              <p className="buyer-response">{result.response}</p>
              <ol className="buyer-tool-log" aria-label="Tool activity">
                {result.activities.map((activity) => (
                  <li key={activity.tool}>
                    <CheckCircle2 aria-hidden="true" />
                    <div><strong>{activity.tool}</strong><span>{activity.detail}</span></div>
                  </li>
                ))}
              </ol>
            </>
          ) : <p className="buyer-placeholder">Submit a request to inspect the live public catalog.</p>}
        </section>
      </div>
    </main>
  );
}
