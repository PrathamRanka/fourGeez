"use client";

import {
  BarChart3,
  Boxes,
  CheckCircle2,
  FileCheck2,
  LifeBuoy,
  Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";

const dashboardLinks = [
  { href: "/dashboard/onboarding", label: "Onboarding", icon: CheckCircle2 },
  { href: "/dashboard/products", label: "Products", icon: Boxes },
  { href: "/dashboard/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/dashboard/transactions", label: "Transactions", icon: FileCheck2 },
] as const;

// DashboardNavigation preserves seller context and marks the current workspace route.
export function DashboardNavigation() {
  const pathname = usePathname();
  const searchParameters = useSearchParams();
  const sellerId = searchParameters.get("sellerId");

  return (
    <>
      <nav aria-label="Seller dashboard">
        {dashboardLinks.map((link) => {
          const Icon = link.icon;
          const href = sellerId
            ? `${link.href}?sellerId=${encodeURIComponent(sellerId)}`
            : link.href;

          return (
            <Link
              key={link.href}
              href={href}
              className="dashboard-nav-link"
              aria-current={pathname === link.href ? "page" : undefined}
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
