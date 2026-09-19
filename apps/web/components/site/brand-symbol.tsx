export const AGENTPAY_SYMBOL_FRAME_PATH =
  "M20 3.5 34.3 11.8v16.4L20 36.5 5.7 28.2V11.8L20 3.5Z";
export const AGENTPAY_SYMBOL_LETTER_PATH =
  "m13.1 22.2 5.1-8.4h4.2l4.5 7.3M15.6 18.3h8.8";

type BrandSymbolProps = {
  className?: string;
};

// BrandSymbol is the shared vector geometry used by the product lockup and app icons.
export function BrandSymbol({ className = "brand-symbol" }: BrandSymbolProps) {
  return (
    <svg viewBox="0 0 40 40" className={className} aria-hidden="true">
      <path
        className="brand-symbol-frame"
        d={AGENTPAY_SYMBOL_FRAME_PATH}
      />
      <path
        className="brand-symbol-letter"
        d={AGENTPAY_SYMBOL_LETTER_PATH}
      />
    </svg>
  );
}
