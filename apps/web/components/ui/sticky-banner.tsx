"use client";

import { X } from "lucide-react";
import {
  motion,
  useMotionValueEvent,
  useReducedMotion,
  useScroll,
} from "motion/react";
import { useState, type ReactNode } from "react";
import { cn } from "@/lib/utils";

type StickyBannerProps = {
  children: ReactNode;
  className?: string;
  dismissible?: boolean;
  hideOnScroll?: boolean;
};

export function StickyBanner({
  children,
  className,
  dismissible = true,
  hideOnScroll = false,
}: StickyBannerProps) {
  const [dismissed, setDismissed] = useState(false);
  const [scrolledPastBanner, setScrolledPastBanner] = useState(false);
  const shouldReduceMotion = useReducedMotion() ?? false;
  const { scrollY } = useScroll();

  useMotionValueEvent(scrollY, "change", (scrollPosition) => {
    if (hideOnScroll) {
      setScrolledPastBanner(scrollPosition > 48);
    }
  });

  if (dismissed) {
    return null;
  }

  const visible = !scrolledPastBanner;

  return (
    <motion.div
      role="status"
      className={cn("sticky-banner", className)}
      initial={false}
      animate={{ opacity: visible ? 1 : 0, y: visible ? 0 : -12 }}
      transition={{
        duration: shouldReduceMotion ? 0 : 0.18,
        ease: "easeOut",
      }}
      aria-hidden={visible ? undefined : true}
    >
      <div className="sticky-banner-content">{children}</div>
      {dismissible ? (
        <button
          type="button"
          className="sticky-banner-dismiss"
          aria-label="Dismiss banner"
          onClick={() => setDismissed(true)}
        >
          <X aria-hidden="true" className="size-4" />
        </button>
      ) : null}
    </motion.div>
  );
}
