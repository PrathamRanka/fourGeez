import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HomepageExperience } from "@/features/marketing/view/homepage-experience";
import { HeroHeadline } from "@/features/marketing/view/hero-headline";

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((complete) => {
    resolve = complete;
  });

  return { promise, resolve };
}

describe("homepage experience handoff", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    Object.defineProperty(window, "IntersectionObserver", {
      configurable: true,
      value: class IntersectionObserver {
        observe() {}
        unobserve() {}
        disconnect() {}
      },
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps progress below 100 and delays the headline reveal until readiness", async () => {
    const fontsReady = deferred();
    Object.defineProperty(document, "readyState", {
      configurable: true,
      value: "loading",
    });
    Object.defineProperty(document, "fonts", {
      configurable: true,
      value: { ready: fontsReady.promise },
    });

    render(
      <HomepageExperience>
        <HeroHeadline />
      </HomepageExperience>,
    );

    const progress = screen.getByRole("progressbar", {
      name: "Preparing AgentPay",
    });
    expect(Number(progress.getAttribute("aria-valuenow"))).toBeLessThan(100);
    expect(
      screen.queryByTestId("hero-headline-reveal"),
    ).not.toBeInTheDocument();

    await act(async () => {
      window.dispatchEvent(new Event("load"));
      await Promise.resolve();
      vi.advanceTimersByTime(2_000);
    });

    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(Number(progress.getAttribute("aria-valuenow"))).toBeLessThan(100);

    await act(async () => {
      fontsReady.resolve();
      await Promise.resolve();
      await Promise.resolve();
      await vi.runAllTimersAsync();
    });

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.getByTestId("hero-headline-reveal")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: "Sell to agents. Settle on-chain.",
      }),
    ).toBeVisible();
  });

  it("bypasses prolonged loading when reduced motion is requested", async () => {
    vi.mocked(window.matchMedia).mockImplementation((query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    render(
      <HomepageExperience>
        <HeroHeadline />
      </HomepageExperience>,
    );

    await act(async () => {
      await Promise.resolve();
    });

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.getByTestId("hero-headline-reveal")).toBeInTheDocument();
  });
});
