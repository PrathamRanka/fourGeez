import {
  BarChart3,
  Boxes,
  CheckCircle2,
  FileCheck2,
  LifeBuoy,
  Settings,
} from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";
import { BrandMark } from "@/components/site/brand-mark";

const dashboardLinks = [
  { href: "/dashboard/onboarding", label: "Onboarding", icon: CheckCircle2 },
  { href: "/dashboard/products", label: "Products", icon: Boxes },
  { href: "/dashboard/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/dashboard/transactions", label: "Transactions", icon: FileCheck2 },
] as const;

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
        <nav aria-label="Seller dashboard">
          {dashboardLinks.map((link) => {
            const Icon = link.icon;

            return (
              <Link
                key={link.href}
                href={link.href}
                className="dashboard-nav-link"
              >
                <Icon aria-hidden="true" />
                {link.label}
              </Link>
            );
          })}
        </nav>
        <div className="dashboard-sidebar-footer">
          <Link href="/docs" className="dashboard-nav-link">
            <LifeBuoy aria-hidden="true" />
            Documentation
          </Link>
          <span className="dashboard-nav-link" aria-disabled="true">
            <Settings aria-hidden="true" />
            Settings
          </span>
        </div>
      </aside>
      <div className="dashboard-workspace">
        <header className="dashboard-topbar">
          <div>
            <span className="dashboard-environment-dot" aria-hidden="true" />
            Demo environment
          </div>
          <span>Seller workspace</span>
        </header>
        <main id="main-content" className="dashboard-main">
          {children}
        </main>
      </div>
    </div>
  );
}
