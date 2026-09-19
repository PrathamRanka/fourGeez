"use client";

import { Bot, Code2, Rocket } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { useId, type ComponentType, type ReactNode } from "react";
import { BrandMark } from "@/components/site/brand-mark";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type VisualContainerProps = {
  children: ReactNode;
  className?: string;
};

type IntegrationItem = {
  id: string;
  icon: ComponentType<{ className?: string }>;
  label: string;
  x: number;
  y: number;
  path: string;
  delay: number;
};

const integrations: IntegrationItem[] = [
  {
    id: "repository",
    icon: Code2,
    label: "Repo",
    x: 105,
    y: 205,
    path: "M 250 205 H 105",
    delay: 0.1,
  },
  {
    id: "ai-ready",
    icon: Bot,
    label: "AI-ready",
    x: 455,
    y: 105,
    path: "M 294 195 V 120 Q 294 105 309 105 H 455",
    delay: 0.2,
  },
  {
    id: "ship",
    icon: Rocket,
    label: "Ship",
    x: 455,
    y: 305,
    path: "M 294 215 V 290 Q 294 305 309 305 H 455",
    delay: 0.3,
  },
];

function AnimatedPath({
  d,
  id,
  delay,
}: {
  d: string;
  id: string;
  delay: number;
}) {
  const reducedMotion = useReducedMotion();

  return (
    <>
      <path
        d={d}
        stroke="currentColor"
        strokeWidth="1"
        fill="none"
        className="text-border"
      />
      <motion.path
        d={d}
        stroke={`url(#${id})`}
        strokeWidth="2"
        fill="none"
        strokeDasharray="40 160"
        initial={{ strokeDashoffset: 200 }}
        animate={reducedMotion ? undefined : { strokeDashoffset: -200 }}
        transition={{
          duration: 4,
          repeat: Infinity,
          ease: "linear",
          delay,
        }}
      />
      <defs>
        <linearGradient id={id} gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="transparent" />
          <stop
            offset="50%"
            stopColor="var(--color-foreground)"
            stopOpacity="0.42"
          />
          <stop offset="100%" stopColor="transparent" />
        </linearGradient>
      </defs>
    </>
  );
}

export function Integration() {
  const containerId = useId().replaceAll(":", "");
  const reducedMotion = useReducedMotion();

  return (
    <div
      className="relative h-full w-full"
      role="region"
      aria-label="AgentPay integration network"
    >
      <svg
        className="pointer-events-none absolute inset-0 h-full w-full"
        viewBox="0 0 564 410"
        fill="none"
        aria-hidden="true"
      >
        {integrations.map((integration) => (
          <AnimatedPath
            key={integration.id}
            d={integration.path}
            id={`${containerId}-${integration.id}`}
            delay={integration.delay}
          />
        ))}
      </svg>

      <div className="absolute top-1/2 left-1/2 z-20 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-2">
        <div className="relative flex items-center justify-center rounded-xl border border-border bg-background p-1.5 shadow-md sm:rounded-2xl sm:p-2 sm:shadow-xl">
          <div className="rounded-lg border border-border bg-card p-2 sm:rounded-xl sm:p-3">
            <BrandMark compact className="[&_.brand-symbol]:!text-[#f2f2ef]" />
          </div>
          <motion.div
            className="absolute inset-0 rounded-xl border-2 border-foreground/10 sm:rounded-2xl"
            animate={
              reducedMotion
                ? undefined
                : { scale: [1, 1.15, 1], opacity: [0.35, 0, 0.35] }
            }
            transition={{ duration: 3, repeat: Infinity }}
          />
        </div>
        <span className="rounded-full border border-border bg-[#080808]/90 px-2.5 py-1 font-mono text-[0.58rem] font-medium tracking-[0.14em] text-foreground uppercase shadow-sm">
          MCP
        </span>
      </div>

      {integrations.map((integration) => {
        const Icon = integration.icon;
        return (
          <motion.div
            key={integration.id}
            initial={reducedMotion ? false : { opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: integration.delay }}
            style={{
              left: `${(integration.x / 564) * 100}%`,
              top: `${(integration.y / 410) * 100}%`,
            }}
            className="absolute z-10 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center gap-2 text-foreground"
            title={integration.label}
          >
            <span className="flex size-8 items-center justify-center rounded-lg border border-border bg-[#080808] shadow-sm sm:size-12 sm:rounded-xl md:size-13.5">
              <Icon className="size-4 sm:size-6" aria-hidden="true" />
            </span>
            <span className="rounded-full border border-border bg-[#080808]/90 px-2.5 py-1 font-mono text-[0.55rem] font-medium tracking-[0.12em] uppercase shadow-sm sm:text-[0.6rem]">
              {integration.label}
            </span>
          </motion.div>
        );
      })}
    </div>
  );
}

export function VisualContainer({ children, className }: VisualContainerProps) {
  return (
    <div
      className={cn(
        "relative flex aspect-[564/460] w-full items-center justify-center overflow-hidden bg-[#080808] p-8 sm:aspect-[564/410]",
        className,
      )}
    >
      <div
        className="absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            "radial-gradient(circle, rgba(90, 90, 90, 0.35) 1px, transparent 1px)",
          backgroundSize: "32px 32px",
        }}
      />
      <div className="pointer-events-none absolute inset-0 bg-linear-to-b from-[#080808]/75 from-10% via-transparent to-90% to-[#080808]/90" />
      <div className="relative z-10 flex h-full w-full items-center justify-center">
        {children}
      </div>
    </div>
  );
}

export function IntegrationCard() {
  return (
    <Card className="flex w-full flex-col gap-0 overflow-hidden rounded-2xl border border-border bg-card p-0 text-card-foreground shadow-sm ring-0">
      <VisualContainer>
        <Integration />
      </VisualContainer>
    </Card>
  );
}

export function IntegrationCardDemo() {
  return <IntegrationCard />;
}

export default IntegrationCardDemo;
