"use client";

import {
  BarChart3,
  Boxes,
  CheckCircle2,
  FileCheck2,
  LayoutDashboard,
  LifeBuoy,
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
] as const;

// DashboardNavigation marks the current route without placing seller identity in URLs.
export function DashboardNavigation() {
  const pathname = usePathname();

  return (
    <>
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
    </>
  );
}
