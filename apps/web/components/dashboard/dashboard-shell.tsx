import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { DashboardNavigation } from "@/components/dashboard/dashboard-navigation";
import { BrandMark } from "@/components/site/brand-mark";
import { SignOutButton } from "@/features/auth/view/sign-out-button";
import styles from "./dashboard-shell.module.css";

type DashboardShellProps = {
  children: ReactNode;
  seller: { email: string; name: string };
};

// DashboardShell provides the persistent seller workspace navigation.
export function DashboardShell({ children, seller }: DashboardShellProps) {
  const sellerInitials = seller.name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((namePart) => namePart[0]?.toUpperCase())
    .join("");

  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#main-content">
        Skip to dashboard content
      </a>
      <aside className={styles.sidebar} aria-label="AgentPay seller workspace">
        <div className={styles.brandBlock}>
          <Link
            href="/"
            aria-label="AgentPay public site"
            className={styles.brand}
          >
            <BrandMark />
          </Link>
        </div>
        <div className={styles.navigationBlock}>
          <span className={styles.navigationLabel}>Workspace</span>
          <Suspense fallback={null}>
            <DashboardNavigation />
          </Suspense>
        </div>
        <div className={styles.sidebarFooter}>
          <span
            className={styles.environment}
            role="status"
            aria-label="Environment"
          >
            <span className={styles.environmentDot} aria-hidden="true" />
            Local testnet
          </span>
          <div className={styles.accountRow}>
            <span className={styles.avatar} aria-hidden="true">
              {sellerInitials || "AP"}
            </span>
            <span className={styles.owner} title={seller.email}>
              <strong>{seller.name}</strong>
              <span>{seller.email}</span>
            </span>
            <SignOutButton />
          </div>
        </div>
      </aside>
      <div className={styles.content}>
        <header className={styles.topbar}>
          <div>
            <span>Seller dashboard</span>
            <strong>Commerce operations</strong>
          </div>
          <span className={styles.topbarStatus}>Live operations</span>
        </header>
        <main id="main-content" className={styles.mainInner}>
          {children}
        </main>
      </div>
    </div>
  );
}
