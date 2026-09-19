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
      screen.getByText(/the next visitor to your site will be an ai agent/i),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Start selling" })).toHaveAttribute(
      "href",
      "/sign-up",
    );
    expect(
      screen.getByRole("link", { name: "Watch agent checkout" }),
    ).toHaveAttribute("href", "/demo/agent-checkout");
    expect(screen.getByTestId("animated-gradient-background")).toHaveAttribute(
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
        name: "Cloud authority. Local freedom.",
      }),
    ).toBeVisible();
    expect(
      screen.getByRole("heading", { name: "One integration. Every sale." }),
    ).toBeVisible();
    expect(screen.getByLabelText("AgentPay integration network")).toBeVisible();
    expect(screen.queryByText("Seller API")).not.toBeInTheDocument();
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

  it("offers a light and dark appearance control", () => {
    render(<HomePage />);

    const themeButton = screen.getByRole("button", {
      name: /switch appearance/i,
    });
    expect(themeButton).toBeVisible();
    fireEvent.click(themeButton);
    expect(document.documentElement).toHaveClass("dark");
  });
});
