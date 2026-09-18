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
            <span className="dashboard-owner">
              <strong>{seller.name}</strong>
              <small>{seller.email}</small>
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
