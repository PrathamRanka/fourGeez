import { cn } from "@/lib/utils";

type BrandMarkProps = {
  className?: string;
  compact?: boolean;
};

// BrandMark renders the AgentPay transaction ring and wordmark.
export function BrandMark({ className, compact = false }: BrandMarkProps) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <svg viewBox="0 0 40 40" className="brand-symbol" aria-hidden="true">
        <path
          className="brand-symbol-frame"
          d="M20 3.5 34.3 11.8v16.4L20 36.5 5.7 28.2V11.8L20 3.5Z"
        />
        <path
          className="brand-symbol-letter"
          d="m13.1 22.2 5.1-8.4h4.2l4.5 7.3M15.6 18.3h8.8"
        />
      </svg>
      {compact ? null : <span className="brand-wordmark">AgentPay</span>}
    </span>
  );
}
