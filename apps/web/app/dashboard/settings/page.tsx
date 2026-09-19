import type { Metadata } from "next";
import { ArrowUpRight, CircleUserRound, Store } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getSellerSession } from "@/features/auth/server/session";
import styles from "./settings.module.css";

export const metadata: Metadata = { title: "Settings" };

export default async function SettingsPage() {
  const session = await getSellerSession();
  if (!session) redirect("/sign-in?returnTo=%2Fdashboard%2Fsettings");

  return (
    <div className={styles.workspace}>
      <header className={styles.pageHeader}>
        <div>
          <p>Workspace preferences</p>
          <h1>Settings</h1>
          <span>Account and store configuration.</span>
        </div>
      </header>

      <div className={styles.settingsGrid}>
        <section className={styles.panel} aria-labelledby="account-settings">
          <header>
            <CircleUserRound aria-hidden="true" />
            <div>
              <p>Identity</p>
              <h2 id="account-settings">Account</h2>
            </div>
          </header>
          <dl className={styles.details}>
            <div>
              <dt>Owner</dt>
              <dd>{session.principal.name}</dd>
            </div>
            <div>
              <dt>Email</dt>
              <dd>{session.principal.email}</dd>
            </div>
          </dl>
        </section>

        <section
          className={`${styles.panel} ${styles.widePanel}`}
          aria-labelledby="store-settings"
        >
          <header>
            <Store aria-hidden="true" />
            <div>
              <p>Commerce</p>
              <h2 id="store-settings">Store and integration</h2>
            </div>
          </header>
          <div className={styles.settingRow}>
            <div>
              <strong>Storefront, wallet, and MCP</strong>
              <span>
                Review the guided setup and production readiness checks.
              </span>
            </div>
            <Link href="/dashboard/onboarding">
              Manage store setup <ArrowUpRight aria-hidden="true" />
            </Link>
          </div>
        </section>
      </div>
    </div>
  );
}
