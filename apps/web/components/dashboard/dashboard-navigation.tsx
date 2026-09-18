"use client";

import {
  BarChart3,
  Boxes,
  CheckCircle2,
  FileCheck2,
  LayoutDashboard,
  LifeBuoy,
  Scale,
  Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

const dashboardLinks = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/dashboard/onboarding", label: "Onboarding", icon: CheckCircle2 },
  { href: "/dashboard/products", label: "Products", icon: Boxes },
  { href: "/dashboard/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/dashboard/transactions", label: "Transactions", icon: FileCheck2 },
  { href: "/dashboard/disputes", label: "Disputes", icon: Scale },
] as const;

// DashboardNavigation marks the current route without placing seller identity in URLs.
export function DashboardNavigation() {
  const pathname = usePathname();

  return (
    <>
      <div className="dashboard-navigation">
        <p className="dashboard-nav-label">Workspace</p>
        <nav aria-label="Seller dashboard">
          {dashboardLinks.map((link) => {
            const Icon = link.icon;
            return (
              <Link
                key={link.href}
                href={link.href}
                className="dashboard-nav-link"
                aria-current={
                  pathname === link.href ||
                  (link.href !== "/dashboard" &&
                    pathname.startsWith(`${link.href}/`))
                    ? "page"
                    : undefined
                }
              >
                <Icon aria-hidden="true" />
                <span>{link.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>
      <div className="dashboard-sidebar-footer">
        <p className="dashboard-nav-label">Resources</p>
        <Link href="/docs" className="dashboard-nav-link">
          <LifeBuoy aria-hidden="true" />
          <span>Documentation</span>
        </Link>
        <span className="dashboard-nav-link" aria-disabled="true">
          <Settings aria-hidden="true" />
          <span>Settings</span>
        </span>
      </div>
    </>
  );
}
