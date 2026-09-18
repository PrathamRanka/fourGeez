"use client";

import {
  Bot,
  Cloud,
  Code2,
  ShieldCheck,
  Store,
  WalletCards,
} from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import Link from "next/link";
import { useId, type ComponentType, type ReactNode } from "react";
import { BrandMark } from "@/components/site/brand-mark";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type VisualContainerProps = {
  children: ReactNode;
  className?: string;
};

type IntegrationCardProps = {
  connected?: boolean;
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
    label: "Seller repository",
    x: 110,
    y: 90,
    path: "M 270 205 V 105 Q 270 90 255 90 H 110",
    delay: 0.1,
  },
  {
    id: "coding-agent",
    icon: Bot,
    label: "Coding agent",
    x: 360,
    y: 70,
    path: "M 294 205 V 85 Q 294 70 309 70 H 360",
    delay: 0.2,
  },
  {
    id: "cloud",
    icon: Cloud,
    label: "AgentPay cloud",
    x: 160,
    y: 205,
    path: "M 250 205 H 160",
    delay: 0.3,
  },
  {
    id: "wallet",
    icon: WalletCards,
    label: "x402 wallet",
    x: 480,
    y: 205,
    path: "M 314 205 H 480",
    delay: 0.4,
  },
  {
    id: "storefront",
    icon: Store,
    label: "Storefront",
    x: 282,
    y: 360,
    path: "M 282 205 V 360",
    delay: 0.6,
  },
  {
    id: "fulfillment",
    icon: ShieldCheck,
    label: "Signed fulfillment",
    x: 460,
    y: 340,
    path: "M 314 215 V 325 Q 314 340 329 340 H 460",
    delay: 0.7,
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
            stopColor="var(--color-primary)"
            stopOpacity="0.6"
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

      <div className="absolute top-1/2 left-1/2 z-20 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-xl border border-border bg-background p-1.5 shadow-md sm:rounded-2xl sm:p-2 sm:shadow-xl">
        <div className="rounded-lg border border-border bg-card p-2 sm:rounded-xl sm:p-3">
          <BrandMark compact className="text-primary" />
        </div>
        <motion.div
          className="absolute inset-0 rounded-xl border-2 border-primary/10 sm:rounded-2xl"
          animate={
            reducedMotion
              ? undefined
              : { scale: [1, 1.15, 1], opacity: [0.35, 0, 0.35] }
          }
          transition={{ duration: 3, repeat: Infinity }}
        />
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
            className="absolute z-10 flex size-8 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-lg border border-border bg-background text-foreground shadow-sm sm:size-12 sm:rounded-xl md:size-13.5"
            title={integration.label}
          >
            <Icon className="size-4 sm:size-6" aria-hidden="true" />
            <span className="sr-only">{integration.label}</span>
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
        "relative flex aspect-[564/460] w-full items-center justify-center overflow-hidden bg-muted p-8 sm:aspect-[564/410]",
        className,
      )}
    >
      <div
        className="absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            "radial-gradient(circle, var(--color-foreground) 1px, transparent 1px)",
          backgroundSize: "32px 32px",
        }}
      />
      <div className="pointer-events-none absolute inset-0 bg-linear-to-b from-background/60 from-10% via-transparent to-90% to-background/60" />
      <div className="relative z-10 flex h-full w-full items-center justify-center">
        {children}
      </div>
    </div>
  );
}

export function IntegrationCard({ connected = true }: IntegrationCardProps) {
  return (
    <Card className="flex w-full flex-col gap-0 overflow-hidden rounded-2xl border border-border bg-card p-0 text-card-foreground shadow-sm ring-0">
      <VisualContainer>
        <Integration />
      </VisualContainer>

      {/* <CardContent className="flex flex-col gap-6 p-6 sm:flex-row sm:items-end sm:justify-between sm:p-8">
        <div className="flex max-w-xl flex-col gap-2">
          <span className="font-mono text-[0.62rem] tracking-[0.12em] text-muted-foreground uppercase">
            {connected ? "Connector active" : "Setup required"}
          </span>
          <h2 className="font-display text-xl font-medium tracking-[-0.035em] sm:text-2xl">
            One MCP. Every surface.
          </h2>
          <p className="text-sm leading-relaxed text-muted-foreground sm:text-base">
            Connect seller code, buyer agents, x402 payments, and signed
            fulfillment through AgentPay.
          </p>
        </div>
        <Button
          nativeButton={false}
          className="h-10 w-fit rounded-full px-5"
          render={<Link href="/dashboard/onboarding" />}
        >
          Manage integration
        </Button>
      </CardContent> */}
    </Card>
  );
}

export function IntegrationCardDemo() {
  return <IntegrationCard />;
}

export default IntegrationCardDemo;
