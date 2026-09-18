import type { Metadata } from "next";
import type { ReactNode } from "react";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteHeader } from "@/components/site/site-header";
import "./globals.css";

const themeInitializationScript = `
try {
  const savedTheme = window.localStorage.getItem("agentpay-theme");
  document.documentElement.classList.toggle("dark", savedTheme === "dark");
} catch {}
`;

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
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{ __html: themeInitializationScript }}
        />
      </head>
      <body>
        <a href="#main-content" className="skip-link">
          Skip to content
        </a>
        <SiteHeader />
        {children}
        <SiteFooter />
      </body>
    </html>
  );
}
