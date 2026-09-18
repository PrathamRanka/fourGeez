import { cn } from "@/lib/utils";

type BrandMarkProps = {
  className?: string;
  compact?: boolean;
};

// BrandMark renders the AgentPay orbit and wordmark.
export function BrandMark({ className, compact = false }: BrandMarkProps) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <span className="relative grid size-7 place-items-center" aria-hidden="true">
        <span className="absolute inset-[0.1875rem] rounded-full border border-current opacity-25" />
        <span className="absolute h-[0.4375rem] w-full rotate-[-28deg] rounded-full border border-current" />
        <span className="size-2 rounded-full bg-current" />
      </span>
      {compact ? null : (
        <span className="font-display text-[1.05rem] font-semibold tracking-[-0.04em]">
          AgentPay
        </span>
      )}
    </span>
  );
}
