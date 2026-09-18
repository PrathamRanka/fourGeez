import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/site/brand-mark";

const footerGroups = [
  {
    label: "Product links",
    title: "Product",
    links: [
      { href: "/demo/agent-checkout", label: "Agent Checkout" },
      { href: "/#product", label: "Revenue Lens" },
      { href: "/#security", label: "Trust Gate" },
    ],
  },
  {
    label: "Company links",
    title: "Resources",
    links: [
      { href: "/docs", label: "Documentation" },
      { href: "/#faq", label: "FAQ" },
      { href: "/sign-up", label: "Get started" },
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

// SiteFooter adapts the MIT Ruixen enterprise footer to AgentPay's public navigation.
export function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="site-container footer-layout">
        <div>
          <BrandMark />
          <p className="footer-description">
            Commerce infrastructure for APIs selling to people and software
            agents.
          </p>
          <div className="footer-proof">
            <ShieldCheck className="size-4" aria-hidden="true" />
            Verified payment. Signed fulfillment.
          </div>
        </div>

        <div className="footer-groups">
          {footerGroups.map((group) => (
            <nav key={group.label} aria-label={group.label}>
              <p className="footer-group-title">{group.title}</p>
              <div className="footer-links">
                {group.links.map((link) => (
                  <Link
                    key={`${group.label}-${link.label}`}
                    href={link.href}
                    className="footer-link"
                  >
                    {link.label}
                  </Link>
                ))}
              </div>
            </nav>
          ))}
        </div>
      </div>

      <div className="site-container footer-bottom">
        <span>© 2026 AgentPay</span>
        <span>Buyer funds settle directly to verified seller wallets.</span>
      </div>
    </footer>
  );
}
