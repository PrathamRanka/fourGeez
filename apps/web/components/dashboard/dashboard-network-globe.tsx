"use client";

import { useSyncExternalStore } from "react";
import { Globe } from "@/components/ui/cobe-globe";
import styles from "./dashboard-visuals.module.css";

export function DashboardNetworkGlobe() {
  const canRenderGlobe = useSyncExternalStore(
    () => () => undefined,
    () => typeof ResizeObserver !== "undefined",
    () => false,
  );

  return (
    <div className={styles.network}>
      {canRenderGlobe ? (
        <Globe label="AgentPay commerce network" />
      ) : (
        <svg className={styles.networkFallback} viewBox="0 0 240 240" role="img" aria-label="AgentPay commerce network">
          <circle cx="120" cy="120" r="88" fill="none" stroke="currentColor" />
          <ellipse cx="120" cy="120" rx="43" ry="88" fill="none" stroke="currentColor" />
          <ellipse cx="120" cy="120" rx="88" ry="35" fill="none" stroke="currentColor" />
          <path d="M45 88c42 18 108 18 150 0M45 152c42-18 108-18 150 0" fill="none" stroke="currentColor" />
          <circle cx="74" cy="91" r="4" fill="var(--dash-blue, #4b73e8)" />
          <circle cx="158" cy="71" r="4" fill="var(--dash-violet, #8776e8)" />
          <circle cx="171" cy="148" r="4" fill="var(--dash-pink, #d99cc0)" />
        </svg>
      )}
      <div className={styles.networkMeta} aria-hidden="true">
        <div><strong>Network reach</strong><span>Agent and browser channels</span></div>
        <span>Live rail</span>
      </div>
    </div>
  );
}
