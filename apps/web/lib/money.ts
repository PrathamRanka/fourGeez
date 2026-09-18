// formatAtomicPrice renders supported six-decimal assets without floating point.
export function formatAtomicPrice(amount: string, asset: string): string {
  if (!/^\d+$/.test(amount)) {
    return `${amount} ${asset}`;
  }

  const paddedAmount = amount.padStart(7, "0");
  const whole = paddedAmount.slice(0, -6).replace(/^0+(?=\d)/, "");
  const fraction = paddedAmount.slice(-6).replace(/0+$/, "");
  return `${fraction ? `${whole}.${fraction}` : whole} ${asset}`;
}
