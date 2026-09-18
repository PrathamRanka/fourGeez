import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DocsPage from "@/app/docs/page";
import PrivacyPage from "@/app/privacy/page";
import SecurityPage from "@/app/security/page";
import SignInPage from "@/app/sign-in/page";
import SignUpPage from "@/app/sign-up/page";
import TermsPage from "@/app/terms/page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), refresh: vi.fn() }),
}));

describe("AgentPay public routes", () => {
  it("provides documentation and account entry pages", async () => {
    render(<DocsPage />);
    expect(
      screen.getByRole("heading", {
        name: "Connect AgentPay to your repository.",
      }),
    ).toBeVisible();

    render(await SignInPage());
    expect(
      screen.getByRole("heading", { name: "Return to your storefront." }),
    ).toBeVisible();

    render(await SignUpPage());
    expect(
      screen.getByRole("heading", { name: "Open your AgentPay storefront." }),
    ).toBeVisible();
  });

  it("provides the public trust and legal pages", () => {
    render(<SecurityPage />);
    expect(
      screen.getByRole("heading", { name: "Verification before execution." }),
    ).toBeVisible();

    render(<PrivacyPage />);
    expect(
      screen.getByRole("heading", { name: "Privacy by minimization." }),
    ).toBeVisible();

    render(<TermsPage />);
    expect(
      screen.getByRole("heading", {
        name: "Clear terms for an early product.",
      }),
    ).toBeVisible();
  });
});
