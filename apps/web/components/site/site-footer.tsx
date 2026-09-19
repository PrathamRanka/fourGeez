import { ArrowUpRight, Code2, ShieldCheck, UserRound } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/site/brand-mark";
import { CloudShader } from "@/components/ui/cloud-shader";

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
  {
    label: "Developer links",
    title: "Developers",
    links: [
      {
        href: "https://github.com/PrathamRanka/fourGeez",
        label: "GitHub repository",
        icon: Code2,
        external: true,
      },
      {
        href: "https://www.linkedin.com/in/prathamranka06/",
        label: "Pratham Ranka on LinkedIn",
        icon: UserRound,
        external: true,
      },
    ],
  },
] as const;

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <div className="site-container footer-cta-wrap">
        <CloudShader
          className="footer-cloud"
          speed={0.14}
          count={4}
          cloudColor="#e5f4ff"
          skyTopColor="#173b7a"
          skyBottomColor="#5aa7d8"
        >
          <div className="footer-cta-content">
            <p className="footer-cta-eyebrow">Agent commerce infrastructure</p>
            <h2>Turn your API into a storefront.</h2>
            <Link href="/sign-up" className="footer-cta-link">
              Create your storefront
              <ArrowUpRight aria-hidden="true" className="size-4" />
            </Link>
          </div>
        </CloudShader>
      </div>

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
                {group.links.map((link) =>
                  "external" in link && link.external ? (
                    <a
                      key={`${group.label}-${link.label}`}
                      href={link.href}
                      className="footer-link"
                      target="_blank"
                      rel="noreferrer"
                    >
                      <link.icon aria-hidden="true" className="size-4" />
                      {link.label}
                    </a>
                  ) : (
                    <Link
                      key={`${group.label}-${link.label}`}
                      href={link.href}
                      className="footer-link"
                    >
                      {link.label}
                    </Link>
                  ),
                )}
              </div>
            </nav>
          ))}
        </div>
      </div>

      <div className="site-container footer-bottom">
        <span>© 2026 AgentPay. Built by Pratham Ranka and Ayush Garg.</span>
        <span>Buyer funds settle directly to verified seller wallets.</span>
      </div>
    </footer>
  );
}
