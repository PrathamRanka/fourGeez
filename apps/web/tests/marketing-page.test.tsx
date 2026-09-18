import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import HomePage from "@/app/page";

vi.mock("@/components/ui/cobe-globe", () => ({
  Globe: () => <div data-testid="commerce-globe" />,
}));

describe("AgentPay public site", () => {
  it("opens with the agent-commerce promise and a single primary action", () => {
    render(<HomePage />);

    expect(
      screen.getByRole("heading", { name: "Sell your API to AI agents." }),
    ).toBeVisible();
    expect(screen.getByText(/one key, one prompt/i)).toBeVisible();
    expect(screen.getByRole("link", { name: "Get started" })).toHaveAttribute(
      "href",
      "/sign-up",
    );
    expect(screen.getByTestId("commerce-globe")).toBeVisible();
  });

  it("shows clear launch, checkout, analytics, and trust demonstrations", () => {
    render(<HomePage />);

    expect(
      screen.getByRole("heading", {
        name: /everything you need to sell to agents/i,
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
        name: "Built around approval, not surprises",
      }),
    ).toBeVisible();
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
      screen.getByText(/does not guarantee search ranking/i),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Start selling with AgentPay" }),
    ).toHaveAttribute("href", "/sign-up");
    expect(
      screen.getByRole("button", { name: "How is AgentPay priced?" }),
    ).toBeVisible();
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
