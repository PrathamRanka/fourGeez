import type { ReactNode } from "react";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";

// DashboardLayout applies the seller workspace chrome to every dashboard route.
export default function DashboardLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return <DashboardShell>{children}</DashboardShell>;
}
