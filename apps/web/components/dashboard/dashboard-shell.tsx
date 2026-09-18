import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { DashboardNavigation } from "@/components/dashboard/dashboard-navigation";
import { BrandMark } from "@/components/site/brand-mark";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";
import { SignOutButton } from "@/features/auth/view/sign-out-button";

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
    <div className="dashboard-shell">
      <aside
        className="dashboard-sidebar"
        aria-label="AgentPay seller workspace"
      >
        <div className="dashboard-sidebar-head">
          <Link
            href="/"
            aria-label="AgentPay public site"
            className="dashboard-brand"
          >
            <BrandMark />
          </Link>
          <span className="dashboard-product-label">Seller OS</span>
        </div>
        <Suspense fallback={null}>
          <DashboardNavigation />
        </Suspense>
      </aside>
      <div className="dashboard-workspace">
        <header className="dashboard-topbar">
          <div className="dashboard-topbar-context">
            <span className="dashboard-topbar-kicker">
              Commerce control plane
            </span>
            <span
              className="dashboard-environment"
              role="status"
              aria-label="Environment"
            >
              <span className="dashboard-environment-dot" aria-hidden="true" />
              Local testnet
            </span>
          </div>
          <div className="dashboard-topbar-actions">
            <span className="dashboard-owner-avatar" aria-hidden="true">
              {sellerInitials || "AP"}
            </span>
            <span className="dashboard-owner">
              <small>Workspace owner</small>
              <strong>{seller.name}</strong>
              <span>{seller.email}</span>
            </span>
            <SignOutButton />
            <ThemeCycleButton />
          </div>
        </header>
        <main id="main-content" className="dashboard-main">
          {children}
        </main>
      </div>
    </div>
  );
}
