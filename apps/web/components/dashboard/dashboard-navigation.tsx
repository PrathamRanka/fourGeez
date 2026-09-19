"use client";

import {
  BarChart3,
  Boxes,
  CheckCircle2,
  FileCheck2,
  LayoutDashboard,
  Scale,
  Settings,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import styles from "./dashboard-shell.module.css";

const dashboardLinks = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/dashboard/onboarding", label: "Onboarding", icon: CheckCircle2 },
  { href: "/dashboard/products", label: "Products", icon: Boxes },
  { href: "/dashboard/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/dashboard/transactions", label: "Transactions", icon: FileCheck2 },
  { href: "/dashboard/disputes", label: "Disputes", icon: Scale },
  { href: "/dashboard/settings", label: "Settings", icon: Settings },
] as const;

// DashboardNavigation marks the current route without placing seller identity in URLs.
export function DashboardNavigation() {
  const pathname = usePathname();

  return (
    <nav className={styles.navigation} aria-label="Seller command navigation">
      {dashboardLinks.map((link) => {
        const Icon = link.icon;
        return (
          <Link
            key={link.href}
            href={link.href}
            className={styles.navLink}
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
  );
}
