export const AGENTPAY_SYMBOL_PATH =
  "M3 40 18 6h9L12 40H3ZM24 6h9l13 17-15 17H20l15-17L24 6Z";

type BrandSymbolProps = {
  className?: string;
};

// BrandSymbol is the shared vector geometry used by the product lockup and app icons.
export function BrandSymbol({ className = "brand-symbol" }: BrandSymbolProps) {
  return (
    <svg viewBox="0 0 48 48" className={className} aria-hidden="true">
      <path
        d={AGENTPAY_SYMBOL_PATH}
        fill="currentColor"
        fillRule="evenodd"
        clipRule="evenodd"
      />
    </svg>
  );
}
