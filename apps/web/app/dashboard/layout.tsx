import type { ReactNode } from "react";
import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { DashboardShell } from "@/components/dashboard/dashboard-shell";
import { safeRelativeReturnPath } from "@/features/auth/policy";
import { getSellerSession } from "@/features/auth/server/session";

// DashboardLayout applies the seller workspace chrome to every dashboard route.
export default async function DashboardLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  const [session, requestHeaders] = await Promise.all([
    getSellerSession(),
    headers(),
  ]);
  if (!session) {
    const returnTo = safeRelativeReturnPath(
      requestHeaders.get("x-agentpay-return-path"),
    );
    redirect(`/sign-in?returnTo=${encodeURIComponent(returnTo)}`);
  }
  return (
    <DashboardShell
      seller={{
        email: session.principal.email,
        name: session.principal.name,
      }}
    >
      {children}
    </DashboardShell>
  );
}
