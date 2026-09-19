"use client";

import { Check, CircleDollarSign, Code2, ShieldCheck } from "lucide-react";
import { useState } from "react";
import styles from "./marketing-page.module.css";

type CommerceView = "buyer" | "seller";

const buyerEvents = [
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

// CommerceDemo keeps the buyer/seller perspective switch inside the new composition.
export function CommerceDemo() {
  const [activeView, setActiveView] = useState<CommerceView>("buyer");

  return (
    <section
      aria-label="Interactive commerce demo"
      className={styles.commerceDemo}
    >
      <div className={styles.commerceHeader}>
        <span>
          <i aria-hidden="true" /> Testnet transaction example
        </span>
        <div
          className={styles.commerceSwitch}
          role="group"
          aria-label="Commerce perspective"
        >
          {(["buyer", "seller"] as const).map((view) => (
            <button
              key={view}
              type="button"
              aria-pressed={activeView === view}
              onClick={() => setActiveView(view)}
            >
              {view === "buyer" ? "Buyer view" : "Seller view"}
            </button>
          ))}
        </div>
      </div>
      {activeView === "buyer" ? (
        <ol className={styles.commerceEvents}>
          {buyerEvents.map(({ icon: Icon, label, detail }, index) => (
            <li key={label}>
              <span>
                <Icon aria-hidden="true" />
              </span>
              <div>
                <strong>{label}</strong>
                <small>{detail}</small>
              </div>
              <time>00:0{index + 1}</time>
            </li>
          ))}
        </ol>
      ) : (
        <div className={styles.sellerView}>
          <p>Revenue Lens</p>
          <strong>84.20</strong>
          <span>USDC · Base Sepolia</span>
          <div>
            <span>Payment finalized</span>
            <code>txn_01K5…8Q2</code>
          </div>
        </div>
      )}
    </section>
  );
}
