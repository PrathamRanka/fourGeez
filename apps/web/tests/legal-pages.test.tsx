import { readFileSync } from "node:fs";
import path from "node:path";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import PrivacyPage, { metadata as privacyMetadata } from "@/app/privacy/page";
import SecurityPage, { metadata as securityMetadata } from "@/app/security/page";
import TermsPage, { metadata as termsMetadata } from "@/app/terms/page";

const UPDATED_DATE = "September 19, 2026";

describe("public legal and security pages", () => {
  it("keeps legal surfaces permanently neutral and free of colored washes", () => {
    const stylesheet = readFileSync(
      path.join(
        process.cwd(),
        "app",
        "privacy",
        "legal-surface.module.css",
      ),
      "utf8",
    );

    expect(stylesheet).not.toMatch(/--brand-(?:blue|pink|orange|violet)/);
    expect(stylesheet).not.toMatch(/(?:linear|radial)-gradient\(/);
  });

  it.each([
    ["Privacy", PrivacyPage, privacyMetadata, "https://agentpay.prathamranka.in/privacy"],
    ["Terms", TermsPage, termsMetadata, "https://agentpay.prathamranka.in/terms"],
    ["Security", SecurityPage, securityMetadata, "https://agentpay.prathamranka.in/security"],
  ])(
    "publishes a canonical, dated %s page with shared legal navigation",
    (_label, Page, metadata, canonical) => {
      render(<Page />);

      expect(screen.getByText("Pre-launch information")).toBeVisible();
      expect(screen.getAllByText(UPDATED_DATE).length).toBeGreaterThan(0);
      expect(
        screen.getByRole("navigation", { name: "Legal pages" }),
      ).toBeVisible();
      expect(screen.getByRole("link", { name: "Privacy" })).toHaveAttribute(
        "href",
        "/privacy",
      );
      expect(screen.getByRole("link", { name: "Terms" })).toHaveAttribute(
        "href",
        "/terms",
      );
      expect(screen.getByRole("link", { name: "Security" })).toHaveAttribute(
        "href",
        "/security",
      );
      expect(metadata.alternates).toEqual({ canonical });
    },
  );

  it("states the current security posture without presenting the preview as production-ready", () => {
    render(<SecurityPage />);

    expect(screen.getByText(/not yet production-ready/i)).toBeVisible();
    expect(screen.getAllByText(/mock and x402 testnet/i).length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText(/never requests or stores wallet private keys/i)).toBeVisible();
    expect(screen.getByText(/external security review/i)).toBeVisible();
  });

  it("describes data handling and unresolved retention obligations", () => {
    render(<PrivacyPage />);

    expect(screen.getByText(/does not custody buyer funds/i)).toBeVisible();
    expect(screen.getByText(/raw payment proofs/i)).toBeVisible();
    expect(screen.getByText(/production retention and deletion periods/i)).toBeVisible();
    expect(screen.getAllByText(/external legal review/i).length).toBeGreaterThan(
      0,
    );
  });

  it("describes the testnet service and refund boundary", () => {
    render(<TermsPage />);

    expect(screen.getByText(/mock or x402 testnet/i)).toBeVisible();
    expect(screen.getByText(/does not execute, custody, or guarantee refunds/i)).toBeVisible();
    expect(screen.getAllByText(/not lawyer-approved/i).length).toBeGreaterThan(
      0,
    );
  });
});
