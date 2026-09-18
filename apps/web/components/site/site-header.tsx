"use client";

import Link from "next/link";
import { Menu, X } from "lucide-react";
import { useState } from "react";
import { BrandMark } from "@/components/site/brand-mark";
import { Button, buttonVariants } from "@/components/ui/button";

const primaryLinks = [
  { href: "/#product", label: "Product" },
  { href: "/#how-it-works", label: "How it works" },
  { href: "/#security", label: "Security" },
  { href: "/docs", label: "Docs" },
] as const;

// SiteHeader provides the shared public navigation shell.
export function SiteHeader() {
  const [menuOpen, setMenuOpen] = useState(false);

  // closeMenu restores the compact navigation after route selection.
  function closeMenu() {
    setMenuOpen(false);
  }

  return (
    <header className="site-header">
      <div className="site-container flex min-h-18 items-center justify-between gap-6">
        <Link
          href="/"
          aria-label="AgentPay home"
          className="rounded-md text-foreground transition-opacity duration-150 ease-out hover:opacity-70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-4"
        >
          <BrandMark />
        </Link>

        <nav aria-label="Primary navigation" className="hidden items-center gap-1 lg:flex">
          {primaryLinks.map((link) => (
            <Link key={link.href} href={link.href} className="nav-link">
              {link.label}
            </Link>
          ))}
        </nav>

        <div className="hidden items-center gap-2 lg:flex">
          <Link href="/sign-in" className={buttonVariants({ variant: "ghost", size: "lg" })}>
            Sign in
          </Link>
          <Link href="/sign-up" className={buttonVariants({ size: "lg" })}>
            Start selling
          </Link>
        </div>

        <Button
          type="button"
          variant="outline"
          size="icon-lg"
          className="lg:hidden"
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
        <nav aria-label="Mobile navigation" className="site-container flex flex-col py-4">
          {primaryLinks.map((link) => (
            <Link key={link.href} href={link.href} className="mobile-nav-link" onClick={closeMenu}>
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
            <Link href="/sign-up" className={buttonVariants({ size: "lg" })} onClick={closeMenu}>
              Start selling
            </Link>
          </div>
        </nav>
      </div>
    </header>
  );
}
