"use client";

import { Bot, Boxes, Code2, Database, Globe2, WalletCards } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import Link from "next/link";
import { useId, type ComponentType, type ReactNode } from "react";
import { cn } from "@/lib/utils";

type IntegrationCardProps = {
  visual: ReactNode;
  title: string;
  description: string;
  href: string;
  className?: string;
};

type IntegrationNode = {
  id: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
  x: number;
  y: number;
  path: string;
  delay: number;
};

const integrationNodes: readonly IntegrationNode[] = [
  {
    id: "storefront",
    label: "Seller storefront",
    icon: Globe2,
    x: 98,
    y: 82,
    path: "M 270 205 V 97 Q 270 82 255 82 H 98",
    delay: 0.05,
  },
  {
    id: "mcp",
    label: "MCP connector",
    icon: Code2,
    x: 418,
    y: 72,
    path: "M 294 205 V 87 Q 294 72 309 72 H 418",
    delay: 0.1,
  },
  {
    id: "agent",
    label: "Buyer agent",
    icon: Bot,
    x: 112,
    y: 224,
    path: "M 250 205 H 132 Q 112 205 112 224",
    delay: 0.15,
  },
  {
    id: "settlement",
    label: "x402 settlement",
    icon: WalletCards,
    x: 462,
    y: 214,
    path: "M 314 205 H 442 Q 462 205 462 214",
    delay: 0.2,
  },
  {
    id: "evidence",
    label: "Evidence store",
    icon: Database,
    x: 180,
    y: 346,
    path: "M 270 225 V 326 Q 270 346 250 346 H 180",
    delay: 0.25,
  },
  {
    id: "fulfillment",
    label: "Fulfillment service",
    icon: Boxes,
    x: 414,
    y: 346,
    path: "M 294 225 V 326 Q 294 346 314 346 H 414",
    delay: 0.3,
  },
];

function AgentPayGlyph() {
  return (
    <svg viewBox="0 0 40 40" aria-hidden="true" className="size-8">
      <path
        d="M20 3.5 34.3 11.8v16.4L20 36.5 5.7 28.2V11.8L20 3.5Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.4"
      />
      <path
        d="m13.1 22.2 5.1-8.4h4.2l4.5 7.3M15.6 18.3h8.8"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2.4"
      />
    </svg>
  );
}

function AnimatedPath({
  path,
  gradientId,
  delay,
  reduceMotion,
}: {
  path: string;
  gradientId: string;
  delay: number;
  reduceMotion: boolean;
}) {
  return (
    <>
      <path
        d={path}
        fill="none"
        stroke="currentColor"
        strokeWidth="1"
        className="text-border"
      />
      <motion.path
        d={path}
        fill="none"
        stroke={`url(#${gradientId})`}
        strokeDasharray="36 124"
        strokeWidth="2"
        initial={false}
        animate={
          reduceMotion ? { strokeDashoffset: 0 } : { strokeDashoffset: -160 }
        }
        transition={
          reduceMotion
            ? { duration: 0 }
            : { duration: 3.8, repeat: Infinity, ease: "linear", delay }
        }
      />
      <defs>
        <linearGradient id={gradientId} gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="transparent" />
          <stop offset="48%" stopColor="var(--color-primary)" />
          <stop offset="100%" stopColor="transparent" />
        </linearGradient>
      </defs>
    </>
  );
}

export function AgentPayIntegrationMap() {
  const rawId = useId();
  const gradientPrefix = rawId.replaceAll(":", "");
  const shouldReduceMotion = useReducedMotion() ?? false;

  return (
    <div
      aria-label="AgentPay integration network"
      className="relative aspect-[564/410] w-full"
      role="img"
    >
      <svg
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 size-full"
        viewBox="0 0 564 410"
      >
        {integrationNodes.map((integrationNode) => (
          <AnimatedPath
            key={integrationNode.id}
            path={integrationNode.path}
            gradientId={`${gradientPrefix}-${integrationNode.id}`}
            delay={integrationNode.delay}
            reduceMotion={shouldReduceMotion}
          />
        ))}
      </svg>

      <div className="integration-hub" aria-hidden="true">
        <span className="integration-hub-core">
          <AgentPayGlyph />
        </span>
      </div>

      {integrationNodes.map((integrationNode) => {
        const Icon = integrationNode.icon;
        return (
          <motion.div
            key={integrationNode.id}
            aria-hidden="true"
            className="integration-node"
            initial={false}
            animate={{ opacity: 1, scale: 1 }}
            transition={{
              duration: shouldReduceMotion ? 0 : 0.2,
              delay: shouldReduceMotion ? 0 : integrationNode.delay,
              ease: "easeOut",
            }}
            style={{
              left: `${(integrationNode.x / 564) * 100}%`,
              top: `${(integrationNode.y / 410) * 100}%`,
            }}
            title={integrationNode.label}
          >
            <Icon className="size-5" />
          </motion.div>
        );
      })}
    </div>
  );
}

export function IntegrationVisual({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("integration-visual", className)}>
      <div className="integration-dot-grid" aria-hidden="true" />
      <div className="relative z-10 size-full">{children}</div>
    </div>
  );
}

export function IntegrationCard({
  visual,
  title,
  description,
  href,
  className,
}: IntegrationCardProps) {
  return (
    <article className={cn("integration-card", className)}>
      <IntegrationVisual>{visual}</IntegrationVisual>
      <div className="integration-card-copy">
        <div>
          <h3>{title}</h3>
          <p>{description}</p>
        </div>
        <Link href={href} className="integration-card-link">
          View documentation
          <span aria-hidden="true">↗</span>
        </Link>
      </div>
    </article>
  );
}
