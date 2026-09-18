import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { BrandMark } from "@/components/site/brand-mark";

const footerGroups = [
  {
    label: "Product links",
    title: "Product",
    links: [
      { href: "/#product", label: "Agent Checkout" },
      { href: "/#discovery", label: "Discovery Mesh" },
      { href: "/#security", label: "Trust Gate" },
    ],
  },
  {
    label: "Company links",
    title: "Explore",
    links: [
      { href: "/docs", label: "Documentation" },
      { href: "/#pricing", label: "Pricing" },
      { href: "/#faq", label: "Questions" },
    ],
  },
  {
    label: "Legal links",
    title: "Legal",
    links: [
      { href: "/privacy", label: "Privacy" },
      { href: "/terms", label: "Terms" },
      { href: "/security", label: "Security" },
    ],
  },
] as const;

// SiteFooter closes public pages with product and trust navigation.
export function SiteFooter() {
  return (
    <footer className="border-t border-border bg-card">
      <div className="site-container grid gap-12 py-[clamp(3rem,7vw,6rem)] lg:grid-cols-[1.4fr_2fr]">
        <div className="max-w-sm">
          <BrandMark />
          <p className="mt-5 text-base leading-7 text-muted-foreground">
            Turn an existing digital service into a storefront that both people and software agents
            can trust.
          </p>
        </div>
        <div className="grid grid-cols-2 gap-8 sm:grid-cols-3">
          {footerGroups.map((group) => (
            <nav key={group.label} aria-label={group.label}>
              <p className="font-mono text-[0.6875rem] font-medium uppercase tracking-[0.16em] text-muted-foreground">
                {group.title}
              </p>
              <div className="mt-4 flex flex-col items-start gap-3">
                {group.links.map((link) => (
                  <Link key={link.href} href={link.href} className="footer-link">
                    {link.label}
                    {link.href === "/docs" ? (
                      <ArrowUpRight className="size-3.5" aria-hidden="true" />
                    ) : null}
                  </Link>
                ))}
              </div>
            </nav>
          ))}
        </div>
      </div>
      <div className="site-container flex flex-col gap-3 border-t border-dashed border-border py-6 font-mono text-[0.6875rem] uppercase tracking-[0.12em] text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <span>AgentPay · Commerce infrastructure for the agent web</span>
        <span>Built for verifiable transactions</span>
      </div>
    </footer>
  );
}
