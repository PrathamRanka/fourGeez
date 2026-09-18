"use client";

import { Check, CircleDollarSign, Code2, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";

type CommerceView = "buyer" | "seller";

// CommerceDemo shows both sides of one shared AgentPay transaction.
export function CommerceDemo() {
  const [activeView, setActiveView] = useState<CommerceView>("buyer");

  return (
    <section aria-label="Interactive commerce demo" className="commerce-demo">
      <div className="commerce-demo-header">
        <div className="commerce-demo-label">
          <span className="commerce-status-dot" />
          Live commerce rail
        </div>
        <div
          className="commerce-view-switch"
          role="group"
          aria-label="Commerce perspective"
        >
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
    {
      icon: Code2,
      label: "Product discovered",
      detail: "research-summary · 0.08 USDC",
    },
    {
      icon: CircleDollarSign,
      label: "Payment required",
      detail: "Exact-price x402 challenge",
    },
    {
      icon: ShieldCheck,
      label: "Payment verified",
      detail: "Replay-safe proof accepted",
    },
    {
      icon: Check,
      label: "Result delivered",
      detail: "Signed fulfillment · 842 ms",
    },
  ] as const;

  return (
    <div className="commerce-buyer-panel">
      <div className="commerce-request-card">
        <div>
          <p className="commerce-eyebrow">Agent request</p>
          <p className="commerce-request-copy">
            “Buy the market brief under ten cents.”
          </p>
        </div>
        <div className="commerce-budget">
          <span>Budget protected</span>
          <span>Maximum 0.10 USDC</span>
        </div>
      </div>
      <ol className="commerce-event-list">
        {events.map((event, index) => {
          const Icon = event.icon;

          return (
            <li key={event.label}>
              <span className="commerce-event-icon">
                <Icon className="size-4" aria-hidden="true" />
              </span>
              <div>
                <p>{event.label}</p>
                <span>{event.detail}</span>
              </div>
              <time>00:0{index + 1}</time>
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
    <div className="commerce-seller-panel">
      <div className="seller-summary">
        <div>
          <p className="commerce-eyebrow">Revenue Lens</p>
          <p className="seller-total">84.20</p>
          <p className="seller-network">USDC · Base Sepolia</p>
        </div>
        <span className="seller-status">Payment finalized</span>
      </div>
      <div className="seller-metrics">
        {[
          ["Fulfilled", "1,024"],
          ["Average", "0.082 USDC"],
          ["Disputed", "0"],
        ].map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <div className="seller-route">
        <div>
          <span>research-summary</span>
          <code>txn_01K5…8Q2</code>
        </div>
        <div className="seller-progress">
          <span />
        </div>
      </div>
    </div>
  );
}
