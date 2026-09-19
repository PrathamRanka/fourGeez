import { BrandSymbol } from "@/components/site/brand-symbol";
import { cn } from "@/lib/utils";

type BrandMarkProps = {
  className?: string;
  compact?: boolean;
};

// BrandMark renders the AgentPay transaction ring and wordmark.
export function BrandMark({ className, compact = false }: BrandMarkProps) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <BrandSymbol />
      {compact ? null : <span className="brand-wordmark">AgentPay</span>}
    </span>
  );
}
