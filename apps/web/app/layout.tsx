import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { ApplicationShell } from "@/components/site/application-shell";
import { SmoothScroll } from "@/components/site/smooth-scroll";
import {
  agentPaySiteOrigin,
  marketingDescription,
} from "@/features/marketing/seo";
import "./globals.css";
import "lenis/dist/lenis.css";

export const metadata: Metadata = {
  metadataBase: new URL(`${agentPaySiteOrigin}/`),
  applicationName: "AgentPay",
  title: {
    default: "AgentPay | Seller-first commerce for APIs",
    template: "%s | AgentPay",
  },
  description: marketingDescription,
  manifest: "/manifest.webmanifest",
  referrer: "origin-when-cross-origin",
  icons: {
    icon: [
      {
        url: "/brand/agentpay-favicon.svg",
        type: "image/svg+xml",
      },
      {
        url: "/favicon.ico",
        sizes: "16x16 32x32",
        type: "image/x-icon",
      },
      {
        url: "/brand/agentpay-icon-32.png",
        sizes: "32x32",
        type: "image/png",
      },
    ],
    apple: [
      {
        url: "/brand/agentpay-icon-180.png",
        sizes: "180x180",
        type: "image/png",
      },
    ],
  },
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
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  );
}
