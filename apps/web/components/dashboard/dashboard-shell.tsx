import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { DashboardNavigation } from "@/components/dashboard/dashboard-navigation";
import { BrandMark } from "@/components/site/brand-mark";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";
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
      <header
        className={styles.header}
        aria-label="AgentPay seller workspace"
      >
        <div className={styles.headerInner}>
          <div className={styles.brandBlock}>
            <Link
              href="/"
              aria-label="AgentPay public site"
              className={styles.brand}
            >
              <BrandMark />
            </Link>
            <span className={styles.productLabel}>Seller network</span>
          </div>
          <Suspense fallback={null}>
            <DashboardNavigation />
          </Suspense>
          <div className={styles.actions}>
            <span
              className={styles.environment}
              role="status"
              aria-label="Environment"
            >
              <span className={styles.environmentDot} aria-hidden="true" />
              Local testnet
            </span>
            <span className={styles.owner} title={seller.email}>
              <small>{sellerInitials || "AP"} / owner</small>
              <strong>{seller.name}</strong>
              <span>{seller.email}</span>
            </span>
            <SignOutButton />
            <ThemeCycleButton />
          </div>
        </div>
      </header>
      <div className={styles.main}>
        <main id="main-content" className={styles.mainInner}>
          {children}
        </main>
      </div>
    </div>
  );
}
