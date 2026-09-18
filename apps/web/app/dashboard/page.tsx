import type { Metadata } from "next";
import { ArrowRight, Boxes, FileCheck2, Settings2 } from "lucide-react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { getSellerSession } from "@/features/auth/server/session";

export const metadata: Metadata = { title: "Seller dashboard" };

const operatingLinks = [
  {
    href: "/dashboard/products",
    label: "Manage products",
    description: "Review pricing, publication status, and storefront details.",
    icon: Boxes,
  },
  {
    href: "/dashboard/transactions",
    label: "Review transactions",
    description: "Inspect payment, delivery proof, receipts, and disputes.",
    icon: FileCheck2,
  },
  {
    href: "/dashboard/onboarding",
    label: "Storefront settings",
    description: "Resume service, payment destination, and coding-agent setup.",
    icon: Settings2,
  },
] as const;

export default async function DashboardPage() {
  const session = await getSellerSession();
  if (!session) redirect("/sign-in?returnTo=%2Fdashboard");
  if (!session.principal.sellerId || !session.principal.onboardingComplete) {
    redirect("/dashboard/onboarding");
  }

  return (
    <div className="dashboard-overview">
      <header>
        <p className="dashboard-eyebrow">Seller workspace</p>
        <h1>{session.principal.name}</h1>
        <p>
          Operate the products buyers can discover, purchase, and receive
          through AgentPay.
        </p>
      </header>
      <section
        aria-label="Seller operations"
        className="dashboard-overview-grid"
      >
        {operatingLinks.map((operation) => {
          const Icon = operation.icon;
          return (
            <Card key={operation.href}>
              <CardHeader>
                <Icon aria-hidden="true" />
                <CardTitle>{operation.label}</CardTitle>
                <CardDescription>{operation.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <Link
                  href={operation.href}
                  className={buttonVariants({ variant: "outline" })}
                >
                  {operation.label} <ArrowRight aria-hidden="true" />
                </Link>
              </CardContent>
            </Card>
          );
        })}
      </section>
    </div>
  );
}
