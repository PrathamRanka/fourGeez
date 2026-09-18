import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { BuyerActivityAction } from "@/features/buyer/model";
import { BuyerActivity } from "@/features/buyer/view/buyer-activity";

describe("buyer activity", () => {
  it("runs storefront discovery and labels deterministic fallback", async () => {
    const run: BuyerActivityAction = vi.fn().mockResolvedValue({
      ok: true,
      value: {
        mode: "deterministic",
        response: "I found one published product that matches your request.",
        activities: [
          { tool: "getStorefrontManifest", status: "completed", detail: "Loaded 1 published route" },
          { tool: "rankEligibleOffers", status: "completed", detail: "Selected /research within the stated budget" },
        ],
      },
    });
    render(<BuyerActivity run={run} />);

    fireEvent.change(screen.getByLabelText("Storefront slug"), { target: { value: "northstar" } });
    fireEvent.change(screen.getByLabelText("What do you need?"), { target: { value: "A market brief under 40 USDC" } });
    fireEvent.submit(screen.getByRole("form", { name: "Buyer request" }));

    await waitFor(() => expect(run).toHaveBeenCalledTimes(1));
    expect(screen.getByText("Deterministic fallback active")).toBeVisible();
    expect(screen.getByText("getStorefrontManifest")).toBeVisible();
    expect(screen.getByText("rankEligibleOffers")).toBeVisible();
    expect(screen.getByText(/one published product/i)).toBeVisible();
  });
});
