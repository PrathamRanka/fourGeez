"use client";

import Lenis from "lenis";
import { useEffect } from "react";

// SmoothScroll adds restrained wheel smoothing while preserving native touch and reduced-motion behavior.
export function SmoothScroll() {
  useEffect(() => {
    const lenis = new Lenis({
      anchors: { duration: 0.9 },
      autoRaf: true,
      duration: 0.9,
      easing: (progress) => 1 - Math.pow(1 - progress, 4),
      overscroll: true,
      respectReducedMotion: true,
      smoothWheel: true,
      stopInertiaOnNavigate: true,
      syncTouch: false,
    });

    return () => lenis.destroy();
  }, []);

  return null;
}
