import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApplicationShell } from "@/components/site/application-shell";

const pathnameState = vi.hoisted(() => ({ value: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => pathnameState.value,
}));

describe("ApplicationShell", () => {
  beforeEach(() => {
    pathnameState.value = "/";
  });

  it("renders the cloud storefront footer on public routes", () => {
    render(
      <ApplicationShell>
        <main>Public content</main>
      </ApplicationShell>,
    );

    expect(
      screen.getByRole("heading", { name: "Turn your API into a storefront." }),
    ).toBeVisible();
  });

  it("keeps dashboard routes footer-free", () => {
    pathnameState.value = "/dashboard";

    render(
      <ApplicationShell>
        <main>Dashboard content</main>
      </ApplicationShell>,
    );

    expect(
      screen.queryByRole("heading", {
        name: "Turn your API into a storefront.",
      }),
    ).not.toBeInTheDocument();
  });
});
