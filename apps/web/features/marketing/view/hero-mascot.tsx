"use client";

import type { DotLottie } from "@lottiefiles/dotlottie-react";
import dynamic from "next/dynamic";
import {
  motion,
  useMotionValue,
  useReducedMotion,
  useSpring,
} from "motion/react";
import { useEffect, useState, type PointerEvent } from "react";

const DotLottieReact = dynamic(
  () =>
    import("@lottiefiles/dotlottie-react").then(
      (module) => module.DotLottieReact,
    ),
  { ssr: false },
);

const MASCOT_SOURCE =
  "https://lottie.host/8cf4ba71-e5fb-44f3-8134-178c4d389417/0CCsdcgNIP.json";

export function HeroMascot({ className }: { className?: string }) {
  const shouldReduceMotion = useReducedMotion();
  const [dotLottie, setDotLottie] = useState<DotLottie | null>(null);
  const x = useMotionValue(0);
  const y = useMotionValue(0);
  const rotate = useMotionValue(0);
  const springConfig = { stiffness: 260, damping: 20, mass: 0.55 };
  const springX = useSpring(x, springConfig);
  const springY = useSpring(y, springConfig);
  const springRotate = useSpring(rotate, springConfig);

  useEffect(() => {
    if (!dotLottie) return;

    if (shouldReduceMotion) {
      dotLottie.pause();
      dotLottie.setFrame(0);
      return;
    }

    dotLottie.play();
  }, [dotLottie, shouldReduceMotion]);

  function handlePointerMove(event: PointerEvent<HTMLDivElement>) {
    if (shouldReduceMotion) return;

    const bounds = event.currentTarget.getBoundingClientRect();
    const horizontalPosition =
      (event.clientX - bounds.left) / bounds.width - 0.5;
    const verticalPosition =
      (event.clientY - bounds.top) / bounds.height - 0.5;

    x.set(horizontalPosition * 14);
    y.set(verticalPosition * 9);
    rotate.set(horizontalPosition * 8);
    dotLottie?.setSpeed(1.35);
  }

  function resetMascot() {
    x.set(0);
    y.set(0);
    rotate.set(0);
    dotLottie?.setSpeed(1);
  }

  return (
    <motion.div
      aria-hidden="true"
      className={className}
      data-testid="hero-mascot"
      onPointerLeave={resetMascot}
      onPointerMove={handlePointerMove}
      style={
        shouldReduceMotion
          ? undefined
          : { rotate: springRotate, x: springX, y: springY }
      }
      whileHover={shouldReduceMotion ? undefined : { scale: 1.08 }}
      whileTap={shouldReduceMotion ? undefined : { scale: 0.96 }}
    >
      <DotLottieReact
        autoplay={false}
        dotLottieRefCallback={setDotLottie}
        loop
        src={MASCOT_SOURCE}
      />
    </motion.div>
  );
}
