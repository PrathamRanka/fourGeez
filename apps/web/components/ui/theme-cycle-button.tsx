"use client";

import { SunMoon } from "lucide-react";
import { Button } from "@/components/ui/button";

// ThemeCycleButton adapts the MIT Ruixen theme control for AgentPay's two supported themes.
export function ThemeCycleButton() {
  // switchAppearance updates the document theme and remembers the explicit visitor choice.
  function switchAppearance() {
    const nextMode = document.documentElement.classList.contains("dark")
      ? "light"
      : "dark";
    document.documentElement.classList.toggle("dark", nextMode === "dark");
    window.localStorage.setItem("agentpay-theme", nextMode);
  }

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-lg"
      aria-label="Switch appearance"
      className="theme-cycle-button"
      onClick={switchAppearance}
    >
      <SunMoon className="size-4" aria-hidden="true" />
    </Button>
  );
}
