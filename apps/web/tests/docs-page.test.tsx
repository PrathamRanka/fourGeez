import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import DocsPage from "@/app/docs/page";

describe("AgentPay developer documentation", () => {
  it("renders a conventional documentation hierarchy with truthful launch status", () => {
    render(<DocsPage />);

    expect(
      screen.getByRole("heading", { name: "Build with AgentPay" }),
    ).toBeVisible();
    const documentationNavigation = screen.getByRole("navigation", {
      name: "Documentation navigation",
    });
    expect(documentationNavigation).toBeVisible();
    expect(
      within(documentationNavigation).getByText("Get started"),
    ).toBeVisible();
    expect(
      within(documentationNavigation).getByText("Core workflow"),
    ).toBeVisible();
    expect(
      within(documentationNavigation).getByText("Reference"),
    ).toBeVisible();
    expect(
      screen.getByRole("navigation", { name: "On this page" }),
    ).toBeVisible();
    expect(screen.getAllByText(/development preview/i)).toHaveLength(2);
    expect(screen.getAllByText(/Base Sepolia USDC/i).length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        /the coding agent edits your repository using AgentPay MCP analysis and guidance/i,
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        /AgentPay MCP does not write repository files/i,
      ),
    ).toBeVisible();
    expect(
      within(documentationNavigation).getByRole("link", { name: "Quickstart" }),
    ).toHaveAttribute("aria-current", "location");
  });

  it("provides an accessible mobile navigation control", () => {
    render(<DocsPage />);

    const navigationButton = screen.getByRole("button", {
      name: "Open documentation navigation",
    });
    expect(navigationButton).toHaveAttribute("aria-expanded", "false");
    fireEvent.click(navigationButton);
    expect(
      screen.getByRole("button", { name: "Close documentation navigation" }),
    ).toHaveAttribute("aria-expanded", "true");
  });

  it("copies setup examples without exposing a real project key", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<DocsPage />);

    const copyButton = screen.getByRole("button", {
      name: "Copy PowerShell setup",
    });
    fireEvent.click(copyButton);

    await waitFor(() => expect(writeText).toHaveBeenCalledOnce());
    expect(writeText.mock.calls[0]?.[0]).toContain("AGENTPAY_PROJECT_KEY");
    expect(writeText.mock.calls[0]?.[0]).not.toMatch(/apc2\.[A-Za-z0-9]/);
    expect(copyButton).toHaveTextContent("Copied");
  });
});
