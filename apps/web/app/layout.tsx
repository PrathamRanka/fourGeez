import type { Metadata } from "next";
import type { ReactNode } from "react";
import { ApplicationShell } from "@/components/site/application-shell";
import { SmoothScroll } from "@/components/site/smooth-scroll";
import "./globals.css";
import "lenis/dist/lenis.css";

export const metadata: Metadata = {
  title: {
    default: "AgentPay — Sell to people and AI agents",
    template: "%s · AgentPay",
  },
  description:
    "Turn an existing API or digital service into a verified storefront for people and AI agents.",
};

// RootLayout applies the shared public product shell and design system.
export default function RootLayout({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <html lang="en" className="dark">
      <body>
        <SmoothScroll />
        <a href="#main-content" className="skip-link">
          Skip to content
        </a>
        <ApplicationShell>{children}</ApplicationShell>
      </body>
    </html>
  );
}
