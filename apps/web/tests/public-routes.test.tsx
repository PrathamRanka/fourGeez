import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DevelopersPage from "@/app/developers/page";
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
      screen.getByRole("navigation", { name: "Documentation sections" }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", {
        name: "Connect AgentPay to your repository.",
      }),
    ).toBeVisible();

    render(await SignInPage());
    expect(
      screen.getByRole("heading", { name: "Welcome back." }),
    ).toBeVisible();

    render(await SignUpPage());
    expect(
      screen.getByRole("heading", { name: "Start selling to agents." }),
    ).toBeVisible();
  });

  it("provides the public trust and legal pages", () => {
    render(<SecurityPage />);
    expect(screen.getByText("Security control index")).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Verification before execution." }),
    ).toBeVisible();

    render(<PrivacyPage />);
    expect(screen.getByText("Data handling index")).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Privacy by minimization." }),
    ).toBeVisible();

    render(<TermsPage />);
    expect(screen.getByText("Commercial boundary index")).toBeVisible();
    expect(
      screen.getByRole("heading", {
        name: "Clear terms for an early product.",
      }),
    ).toBeVisible();
  });

  it("credits the two AgentPay developers with verified public profiles", () => {
    render(<DevelopersPage />);

    expect(
      screen.getByRole("heading", { name: "Built by people who ship." }),
    ).toBeVisible();
    expect(screen.getByRole("heading", { name: "Pratham Ranka" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Ayush Garg" })).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Pratham Ranka on GitHub" }),
    ).toHaveAttribute("href", "https://github.com/PrathamRanka");
    expect(
      screen.getByRole("link", { name: "Pratham Ranka on LinkedIn" }),
    ).toHaveAttribute("href", "https://www.linkedin.com/in/prathamranka06/");
    expect(
      screen.getByRole("link", { name: "Ayush Garg on GitHub" }),
    ).toHaveAttribute("href", "https://github.com/gargayush1911");
    expect(screen.getByText("LinkedIn profile not published")).toBeVisible();
  });
});
