import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RecoveryPage from "@/app/recover/page";
import SignInPage from "@/app/sign-in/page";
import SignUpPage from "@/app/sign-up/page";
import VerifyPage from "@/app/verify/page";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, refresh: vi.fn() }),
}));

describe("seller account screens", () => {
  beforeEach(() => {
    push.mockReset();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ csrfToken: "csrf-token" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  });

  it("collects real seller registration and verification details", async () => {
    render(await SignUpPage());
    expect(screen.getByRole("heading", { name: "Create account" })).toBeVisible();
    expect(screen.getByLabelText("Your name")).toBeVisible();
    expect(screen.getByLabelText("Work email")).toBeVisible();
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute(
      "autocomplete",
      "new-password",
    );
    expect(
      screen.getByRole("button", { name: "Create seller account" }),
    ).toBeVisible();

    render(await VerifyPage());
    expect(screen.getByText("Identity checkpoint")).toBeVisible();
    expect(screen.getByLabelText("Verification code")).toBeVisible();
    expect(screen.getByRole("button", { name: "Verify email" })).toBeVisible();
  });

  it("shows a safe sign-in error and restores the requested dashboard path", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ csrfToken: "csrf-token" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ error: "Email or password is incorrect." }),
          {
            status: 401,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

    render(
      await SignInPage({
        searchParams: Promise.resolve({ returnTo: "/dashboard/products" }),
      }),
    );
    expect(screen.getByRole("heading", { name: "Sign in" })).toBeVisible();
    fireEvent.change(screen.getByLabelText("Work email"), {
      target: { value: "owner@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "wrong-password" },
    });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Sign in" })).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Email or password is incorrect.",
    );
    expect(push).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(fetch).toHaveBeenLastCalledWith(
        "/api/auth/sign-in",
        expect.objectContaining({
          body: JSON.stringify({
            email: "owner@example.com",
            password: "wrong-password",
            returnTo: "/dashboard/products",
          }),
          headers: expect.objectContaining({
            "X-AgentPay-CSRF": "csrf-token",
          }),
        }),
      );
    });
  });

  it("provides both recovery steps without exposing a credential in the URL", () => {
    render(<RecoveryPage />);
    expect(screen.getByText("Two-step recovery")).toBeVisible();
    expect(screen.getByLabelText("Work email")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Send recovery code" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Recovery code")).toBeVisible();
    expect(screen.getByLabelText("New password")).toBeVisible();
  });
});
