import { cn } from "@/lib/utils";

type BrandMarkProps = {
  className?: string;
  compact?: boolean;
};

// BrandMark renders the AgentPay transaction ring and wordmark.
export function BrandMark({ className, compact = false }: BrandMarkProps) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <span className="brand-symbol" aria-hidden="true">
        <span />
      </span>
      {compact ? null : (
        <span className="font-display text-[1.05rem] font-semibold tracking-[-0.045em]">
          AgentPay
        </span>
      )}
    </span>
  );
}
