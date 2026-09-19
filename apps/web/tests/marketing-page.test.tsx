import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import HomePage from "@/app/page";

describe("AgentPay public site", () => {
  it("opens with the agent-commerce promise and a focused launch path", () => {
    render(<HomePage />);

    expect(
      screen.getByRole("heading", { name: "Sell to agents. Settle on-chain." }),
    ).toBeVisible();
    expect(
      screen.getByText(/give people and software agents one storefront/i),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Start selling" })).toHaveAttribute(
      "href",
      "/sign-up",
    );
    expect(
      screen.queryByRole("link", { name: "Watch agent checkout" }),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("animated-gradient-background")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
    expect(screen.getByTestId("hero-mascot")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("shows the bounded product, integration, and network story", () => {
    render(<HomePage />);

    expect(
      screen.getByRole("heading", {
        name: "Your route to revenue.",
      }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Launch with one prompt" }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Agents pay. Your API responds." }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "Track every verified sale" }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", {
        name: "Ship with machine-readable discovery",
      }),
    ).toBeVisible();
    expect(screen.getByText("llms.txt")).toBeVisible();
    expect(screen.getByText("Structured data")).toBeVisible();
    expect(
      screen.getByRole("heading", {
        name: "Cloud authority. Local freedom.",
      }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "One integration. Every sale." }),
    ).toBeVisible();
    expect(screen.getByLabelText("AgentPay integration network")).toBeVisible();
    expect(screen.queryByText("Seller API")).not.toBeInTheDocument();
  });

  it("groups the maintained stack matrix into distinct ecosystem marks", () => {
    render(<HomePage />);

    const stackStrip = screen.getByRole("region", {
      name: "13 supported ecosystems covering 21 maintained stacks",
    });
    expect(stackStrip).toHaveAttribute("data-slot", "marquee");
    expect(screen.getByText("21", { selector: "strong" })).toBeVisible();
    expect(stackStrip.querySelectorAll("svg")).toHaveLength(26);
    for (const ecosystem of [
      "Next.js",
      "React / Vite",
      "Node.js",
      "Go",
      "Python",
      ".NET",
      "Java",
      "Ruby",
      "PHP",
    ]) {
      expect(within(stackStrip).getAllByLabelText(ecosystem)).toHaveLength(2);
      expect(within(stackStrip).queryByText(ecosystem)).not.toBeInTheDocument();
    }
    expect(within(stackStrip).queryByLabelText("Gin")).not.toBeInTheDocument();
    expect(within(stackStrip).queryByLabelText("Echo")).not.toBeInTheDocument();
    expect(
      within(stackStrip).queryByLabelText("Fiber"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Perl")).not.toBeInTheDocument();
  });

  it("uses the full viewport width without a framed landing-page gutter", () => {
    render(<HomePage />);

    expect(screen.getByRole("main").firstElementChild).toHaveAttribute(
      "data-full-width",
      "true",
    );
  });

  it("switches the commerce demonstration between buyer and seller views", () => {
    render(<HomePage />);

    const demonstration = screen.getByRole("region", {
      name: "Interactive commerce demo",
    });
    expect(within(demonstration).getByText("Payment required")).toBeVisible();
    fireEvent.click(
      within(demonstration).getByRole("button", { name: "Seller view" }),
    );
    expect(within(demonstration).getByText("Revenue Lens")).toBeVisible();
    expect(within(demonstration).getByText("Payment finalized")).toBeVisible();
  });

  it("states payment and discovery limits truthfully", () => {
    render(<HomePage />);

    expect(
      screen.getByText(/funds settle directly to your verified wallet/i),
    ).toBeVisible();
    expect(
      screen.getByText(/without taking custody of buyer funds/i),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Create your storefront" }),
    ).toHaveAttribute("href", "/sign-up");
    expect(
      screen.getByRole("button", { name: "How is AgentPay priced?" }),
    ).toBeVisible();
  });

  it("presents the implemented plans without inventing subscription prices", () => {
    render(<HomePage />);

    const pricing = screen.getByRole("region", { name: "AgentPay plans" });
    expect(
      within(pricing).getByRole("heading", { name: "Starter" }),
    ).toBeVisible();
    expect(
      within(pricing).getByRole("heading", { name: "Growth" }),
    ).toBeVisible();
    expect(
      within(pricing).getByRole("heading", { name: "Scale" }),
    ).toBeVisible();
    expect(within(pricing).getByText("5 published products")).toBeVisible();
    expect(within(pricing).getByText("50 published products")).toBeVisible();
    expect(within(pricing).getByText("500 published products")).toBeVisible();
    expect(within(pricing).queryByText(/\$\d/)).not.toBeInTheDocument();
  });

  it("keeps the public experience dark without an appearance control", () => {
    render(<HomePage />);

    expect(
      screen.queryByRole("button", { name: /switch appearance/i }),
    ).not.toBeInTheDocument();
  });
});
