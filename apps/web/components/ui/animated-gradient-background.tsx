"use client";

import { motion, useReducedMotion } from "motion/react";
import { useEffect, useRef, type CSSProperties } from "react";
import { cn } from "@/lib/utils";

const DEFAULT_GRADIENT_COLORS = [
  "#0A0A0A",
  "#2979FF",
  "#FF5AA5",
  "#FF6D00",
] as const;

const DEFAULT_GRADIENT_STOPS = [35, 50, 60, 70] as const;

interface AnimatedGradientBackgroundProps {
  startingGap?: number;
  breathing?: boolean;
  gradientColors?: readonly string[];
  gradientStops?: readonly number[];
  animationSpeed?: number;
  breathingRange?: number;
  containerStyle?: CSSProperties;
  containerClassName?: string;
  topOffset?: number;
}

function createGradient(
  width: number,
  topOffset: number,
  gradientColors: readonly string[],
  gradientStops: readonly number[],
) {
  const gradientStopsString = gradientStops
    .map((stop, index) => `${gradientColors[index]} ${stop}%`)
    .join(", ");

  return `radial-gradient(${width}% ${width + topOffset}% at 50% 20%, ${gradientStopsString})`;
}

export default function AnimatedGradientBackground({
  startingGap = 125,
  breathing = false,
  gradientColors = DEFAULT_GRADIENT_COLORS,
  gradientStops = DEFAULT_GRADIENT_STOPS,
  animationSpeed = 0.02,
  breathingRange = 5,
  containerStyle,
  topOffset = 0,
  containerClassName,
}: AnimatedGradientBackgroundProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const shouldReduceMotion = useReducedMotion();

  if (gradientColors.length !== gradientStops.length) {
    throw new Error(
      `gradientColors and gradientStops must have the same length. Received ${gradientColors.length} colors and ${gradientStops.length} stops.`,
    );
  }

  const initialGradient = createGradient(
    startingGap,
    topOffset,
    gradientColors,
    gradientStops,
  );

  useEffect(() => {
    if (!breathing || shouldReduceMotion) {
      return;
    }

    let animationFrame = 0;
    let width = startingGap;
    let direction = 1;

    const animateGradient = () => {
      if (width >= startingGap + breathingRange) direction = -1;
      if (width <= startingGap - breathingRange) direction = 1;

      width += direction * animationSpeed;

      if (containerRef.current) {
        containerRef.current.style.background = createGradient(
          width,
          topOffset,
          gradientColors,
          gradientStops,
        );
      }

      animationFrame = requestAnimationFrame(animateGradient);
    };

    animationFrame = requestAnimationFrame(animateGradient);
    return () => cancelAnimationFrame(animationFrame);
  }, [
    animationSpeed,
    breathing,
    breathingRange,
    gradientColors,
    gradientStops,
    shouldReduceMotion,
    startingGap,
    topOffset,
  ]);

  return (
    <motion.div
      aria-hidden="true"
      className={cn(
        "pointer-events-none absolute inset-0 overflow-hidden",
        containerClassName,
      )}
      data-testid="animated-gradient-background"
      initial={false}
    >
      <div
        className="absolute inset-0"
        ref={containerRef}
        style={{ ...containerStyle, background: initialGradient }}
      />
    </motion.div>
  );
}
