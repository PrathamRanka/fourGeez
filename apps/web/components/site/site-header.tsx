"use client";

import { Menu, X } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { BrandMark } from "@/components/site/brand-mark";
import { Button, buttonVariants } from "@/components/ui/button";
import { StickyBanner } from "@/components/ui/sticky-banner";
import { ThemeCycleButton } from "@/components/ui/theme-cycle-button";

const primaryLinks = [
  { href: "/#product", label: "Product" },
  { href: "/#how-it-works", label: "How it works" },
  { href: "/#security", label: "Security" },
  { href: "/docs", label: "Docs" },
] as const;

// SiteHeader provides the compact public navigation and launch announcement.
export function SiteHeader() {
  const [menuOpen, setMenuOpen] = useState(false);

  // closeMenu restores the compact navigation after route selection.
  function closeMenu() {
    setMenuOpen(false);
  }

  return (
    <header className="site-header">
      <StickyBanner className="announcement-bar" hideOnScroll>
        <span>Testnet storefronts are open</span>
        <Link className="announcement-link" href="/docs">
          Read the launch guide <span aria-hidden="true">↗</span>
        </Link>
      </StickyBanner>

      <div className="site-header-rail">
        <div className="site-container flex min-h-16 items-center justify-between gap-6">
          <Link href="/" aria-label="AgentPay home" className="brand-link">
            <BrandMark />
          </Link>

          <nav
            aria-label="Primary navigation"
            className="hidden items-center gap-1 lg:flex"
          >
            {primaryLinks.map((link) => (
              <Link key={link.href} href={link.href} className="nav-link">
                {link.label}
              </Link>
            ))}
          </nav>

          <div className="hidden items-center gap-1.5 lg:flex">
            <ThemeCycleButton />
            <Link
              href="/sign-in"
              className={buttonVariants({
                variant: "outline",
                size: "lg",
                className: "rounded-full px-4",
              })}
            >
              Sign in
            </Link>
            <Link
              href="/sign-up"
              className={buttonVariants({
                size: "lg",
                className: "rounded-full px-4 shadow-sm",
              })}
            >
              Get started
            </Link>
          </div>

          <Button
            type="button"
            variant="outline"
            size="icon-lg"
            className="rounded-full lg:hidden"
            aria-label={menuOpen ? "Close navigation" : "Open navigation"}
            aria-expanded={menuOpen}
            aria-controls="mobile-navigation"
            onClick={() => setMenuOpen((currentState) => !currentState)}
          >
            {menuOpen ? <X aria-hidden="true" /> : <Menu aria-hidden="true" />}
          </Button>
        </div>

        <div
          id="mobile-navigation"
          data-open={menuOpen}
          className="mobile-navigation lg:hidden"
        >
          <nav
            aria-label="Mobile navigation"
            className="site-container flex flex-col py-4"
          >
            {primaryLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className="mobile-nav-link"
                onClick={closeMenu}
              >
                {link.label}
              </Link>
            ))}
            <div className="mt-4 grid grid-cols-2 gap-2 border-t border-border pt-4">
              <Link
                href="/sign-in"
                className={buttonVariants({ variant: "outline", size: "lg" })}
                onClick={closeMenu}
              >
                Sign in
              </Link>
              <Link
                href="/sign-up"
                className={buttonVariants({ size: "lg" })}
                onClick={closeMenu}
              >
                Get started
              </Link>
            </div>
          </nav>
        </div>
      </div>
    </header>
  );
}
