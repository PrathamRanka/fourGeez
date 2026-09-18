"use client";

import type { Arc, COBEOptions, Globe as CobeInstance, Marker } from "cobe";
import { useEffect, useRef } from "react";
import { cn } from "@/lib/utils";

type GlobeProps = {
  arcs?: Arc[];
  className?: string;
  label?: string;
  markers?: Marker[];
};

const noArcs: Arc[] = [];
const noMarkers: Marker[] = [];

// Globe renders the MIT-licensed COBE visualization selected from 21st.dev.
// Source reference: https://21st.dev/@shuding/components/cobe-globe
export function Globe({
  arcs = noArcs,
  className,
  label = "Live AgentPay commerce network",
  markers = noMarkers,
}: GlobeProps) {
  const canvasReference = useRef<HTMLCanvasElement>(null);
  const rotationReference = useRef(0);
  const dragStartReference = useRef<number | null>(null);
  const dragMovementReference = useRef(0);

  useEffect(() => {
    const canvas = canvasReference.current;
    if (!canvas) {
      return;
    }

    let cancelled = false;
    let globe: CobeInstance | undefined;
    let width = canvas.clientWidth;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const resizeObserver = new ResizeObserver((entries) => {
      width = entries[0]?.contentRect.width ?? canvas.clientWidth;
    });
    resizeObserver.observe(canvas);

    void import("cobe").then(({ default: createGlobe }) => {
      if (cancelled) {
        return;
      }
      const options = {
        width: Math.max(width, 1) * Math.min(window.devicePixelRatio, 2),
        height: Math.max(width, 1) * Math.min(window.devicePixelRatio, 2),
        devicePixelRatio: Math.min(window.devicePixelRatio, 2),
        phi: 0.15,
        theta: 0.22,
        dark: 0,
        diffuse: 1.2,
        mapSamples: 16_000,
        mapBrightness: 6,
        baseColor: [1, 1, 1] as [number, number, number],
        markerColor: [0.37, 0.22, 0.94] as [number, number, number],
        glowColor: [0.88, 0.91, 1] as [number, number, number],
        arcColor: [0.18, 0.76, 0.56] as [number, number, number],
        arcWidth: 0.7,
        arcHeight: 0.22,
        markerElevation: 0.025,
        markers,
        arcs,
        onRender: (state: Record<string, number>) => {
          if (!reducedMotion && dragStartReference.current === null) {
            rotationReference.current += 0.0024;
          }
          state.phi = rotationReference.current + dragMovementReference.current;
          state.width = Math.max(width, 1) * Math.min(window.devicePixelRatio, 2);
          state.height = Math.max(width, 1) * Math.min(window.devicePixelRatio, 2);
        },
      } satisfies COBEOptions & {
        onRender: (state: Record<string, number>) => void;
      };
      globe = createGlobe(canvas, options);
    });

    return () => {
      cancelled = true;
      resizeObserver.disconnect();
      globe?.destroy();
    };
  }, [arcs, markers]);

  // startDragging captures the pointer position without triggering React renders.
  function startDragging(event: React.PointerEvent<HTMLCanvasElement>) {
    dragStartReference.current = event.clientX;
    event.currentTarget.setPointerCapture(event.pointerId);
  }

  // continueDragging rotates the globe while preserving animation performance.
  function continueDragging(event: React.PointerEvent<HTMLCanvasElement>) {
    if (dragStartReference.current === null) {
      return;
    }
    dragMovementReference.current = (event.clientX - dragStartReference.current) / 180;
  }

  // stopDragging commits the latest manual rotation.
  function stopDragging() {
    rotationReference.current += dragMovementReference.current;
    dragMovementReference.current = 0;
    dragStartReference.current = null;
  }

  return (
    <canvas
      ref={canvasReference}
      role="img"
      aria-label={label}
      className={cn("aspect-square w-full cursor-grab touch-none active:cursor-grabbing", className)}
      onPointerDown={startDragging}
      onPointerMove={continueDragging}
      onPointerUp={stopDragging}
      onPointerCancel={stopDragging}
    />
  );
}
