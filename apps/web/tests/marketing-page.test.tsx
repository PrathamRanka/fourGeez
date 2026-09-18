import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import HomePage from "@/app/page";

vi.mock("@/components/ui/cobe-globe", () => ({
  Globe: () => <div data-testid="commerce-globe" />,
}));

describe("AgentPay public site", () => {
  it("explains the product to sellers without protocol knowledge", () => {
    render(<HomePage />);

    expect(
      screen.getByRole("heading", { name: "Turn your API into a storefront agents can buy from." }),
    ).toBeVisible();
    expect(screen.getByText(/one key and one prompt/i)).toBeVisible();
    expect(screen.getByRole("heading", { name: "Launch Rail" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Agent Checkout" })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Proof Stream" })).toBeVisible();
  });

  it("shows the complete seller launch sequence in order", () => {
    render(<HomePage />);

    const workflow = screen.getByRole("region", { name: "How AgentPay launches a storefront" });
    const steps = within(workflow).getAllByRole("listitem");
    expect(steps).toHaveLength(4);
    expect(steps[0]).toHaveTextContent("Connect");
    expect(steps[1]).toHaveTextContent("Generate");
    expect(steps[2]).toHaveTextContent("Verify");
    expect(steps[3]).toHaveTextContent("Sell");
  });

  it("provides a working buyer and seller commerce demonstration", () => {
    render(<HomePage />);

    const demonstration = screen.getByRole("region", { name: "Interactive commerce demo" });
    expect(within(demonstration).getByText("Payment required")).toBeVisible();
    fireEvent.click(within(demonstration).getByRole("button", { name: "Seller view" }));
    expect(within(demonstration).getByText("Revenue Lens")).toBeVisible();
    expect(within(demonstration).getByText("Payment finalized")).toBeVisible();
  });

  it("states payment and discovery limits truthfully", () => {
    render(<HomePage />);

    expect(screen.getByText(/funds settle directly to your verified wallet/i)).toBeVisible();
    expect(screen.getByText(/does not guarantee search ranking/i)).toBeVisible();
    expect(screen.getByRole("link", { name: "Start selling with AgentPay" })).toHaveAttribute(
      "href",
      "/sign-up",
    );
  });
});
