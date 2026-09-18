import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { DashboardNavigation } from "@/components/dashboard/dashboard-navigation";
import { BrandMark } from "@/components/site/brand-mark";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";

type DashboardShellProps = {
  children: ReactNode;
};

// DashboardShell provides the persistent seller workspace navigation.
export function DashboardShell({ children }: DashboardShellProps) {
  return (
    <div className="dashboard-shell">
      <aside className="dashboard-sidebar">
        <Link
          href="/"
          aria-label="AgentPay public site"
          className="dashboard-brand"
        >
          <BrandMark />
        </Link>
        <Suspense fallback={null}>
          <DashboardNavigation />
        </Suspense>
      </aside>
      <div className="dashboard-workspace">
        <header className="dashboard-topbar">
          <div>
            <span className="dashboard-environment-dot" aria-hidden="true" />
            Demo environment
          </div>
          <div>
            <span>Seller workspace</span>
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
