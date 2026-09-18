"use client";

import { Check, CircleDollarSign, Code2, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";

type CommerceView = "buyer" | "seller";

// CommerceDemo shows both sides of one shared AgentPay transaction.
export function CommerceDemo() {
  const [activeView, setActiveView] = useState<CommerceView>("buyer");

  return (
    <section
      aria-label="Interactive commerce demo"
      className="commerce-demo overflow-hidden rounded-[1.75rem] border border-white/70 bg-white/70 shadow-[var(--shadow-float)] backdrop-blur-xl"
    >
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4 sm:px-6">
        <div className="flex items-center gap-2 font-mono text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
          <span className="size-2 rounded-full bg-emerald-500 shadow-[0_0_0_0.3rem_rgba(16,185,129,0.12)]" />
          Live commerce rail
        </div>
        <div className="flex rounded-xl bg-muted p-1" role="group" aria-label="Commerce perspective">
          <Button
            type="button"
            size="sm"
            variant={activeView === "buyer" ? "default" : "ghost"}
            aria-pressed={activeView === "buyer"}
            onClick={() => setActiveView("buyer")}
          >
            Buyer view
          </Button>
          <Button
            type="button"
            size="sm"
            variant={activeView === "seller" ? "default" : "ghost"}
            aria-pressed={activeView === "seller"}
            onClick={() => setActiveView("seller")}
          >
            Seller view
          </Button>
        </div>
      </div>

      {activeView === "buyer" ? <BuyerPanel /> : <SellerPanel />}
    </section>
  );
}

// BuyerPanel presents the machine purchase sequence in plain language.
function BuyerPanel() {
  const events = [
    { icon: Code2, label: "Product discovered", detail: "research-summary · $0.08 USDC" },
    { icon: CircleDollarSign, label: "Payment required", detail: "Exact-price x402 challenge" },
    { icon: ShieldCheck, label: "Payment verified", detail: "Replay-safe proof accepted" },
    { icon: Check, label: "Result delivered", detail: "Signed fulfillment · 842 ms" },
  ] as const;

  return (
    <div className="grid min-h-[23rem] gap-8 p-5 sm:p-7 lg:grid-cols-[0.8fr_1.2fr]">
      <div className="flex flex-col justify-between rounded-2xl bg-foreground p-6 text-background">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.15em] text-background/55">Agent request</p>
          <p className="mt-5 font-display text-2xl font-semibold tracking-[-0.04em]">
            “Buy the market brief under ten cents.”
          </p>
        </div>
        <div className="mt-10 flex items-center justify-between border-t border-background/15 pt-5 font-mono text-xs text-background/65">
          <span>Budget protected</span>
          <span>Maximum $0.10</span>
        </div>
      </div>
      <ol className="flex flex-col justify-center">
        {events.map((event, index) => {
          const Icon = event.icon;
          return (
            <li key={event.label} className="group flex gap-4 py-3.5 not-last:border-b not-last:border-border">
              <span className="grid size-9 shrink-0 place-items-center rounded-full border border-border bg-card text-primary">
                <Icon className="size-4" aria-hidden="true" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-3">
                  <p className="font-display text-sm font-semibold">{event.label}</p>
                  <span className="font-mono text-[0.6875rem] text-muted-foreground">
                    00:0{index + 1}
                  </span>
                </div>
                <p className="mt-1 text-sm text-muted-foreground">{event.detail}</p>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

// SellerPanel presents one reconciled sale without combining currencies.
function SellerPanel() {
  return (
    <div className="min-h-[23rem] p-5 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4 border-b border-border pb-6">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.14em] text-muted-foreground">
            Revenue Lens
          </p>
          <p className="mt-2 font-display text-[clamp(2.25rem,6vw,4rem)] font-semibold tracking-[-0.06em]">
            84.20
          </p>
          <p className="text-sm text-muted-foreground">USDC · Base Sepolia</p>
        </div>
        <span className="rounded-full bg-emerald-50 px-3 py-1.5 font-mono text-xs font-medium text-emerald-700">
          Payment finalized
        </span>
      </div>
      <div className="grid gap-3 pt-6 sm:grid-cols-3">
        {[
          ["Fulfilled", "1,024"],
          ["Average", "0.082 USDC"],
          ["Disputed", "0"],
        ].map(([label, value]) => (
          <div key={label} className="rounded-2xl border border-border bg-card p-4">
            <p className="font-mono text-[0.6875rem] uppercase tracking-[0.12em] text-muted-foreground">
              {label}
            </p>
            <p className="mt-3 font-display text-xl font-semibold tracking-[-0.03em]">{value}</p>
          </div>
        ))}
      </div>
      <div className="mt-3 rounded-2xl border border-border bg-card p-4">
        <div className="flex items-center justify-between gap-4 text-sm">
          <span>research-summary</span>
          <span className="font-mono text-muted-foreground">txn_01K5…8Q2</span>
        </div>
        <div className="mt-4 h-2 overflow-hidden rounded-full bg-muted">
          <div className="h-full w-[84%] origin-left rounded-full bg-primary commerce-progress" />
        </div>
      </div>
    </div>
  );
}
