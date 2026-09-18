export const supportedAssetDecimals = 6;

function assertValidDecimals(decimals: number) {
  if (!Number.isInteger(decimals) || decimals < 0) {
    throw new RangeError("Asset decimals must be a non-negative integer.");
  }
}

// formatAtomicUnits renders an integer amount as an exact decimal string.
export function formatAtomicUnits(
  amount: string,
  decimals = supportedAssetDecimals,
): string {
  assertValidDecimals(decimals);
  if (!/^\d+$/.test(amount)) {
    return amount;
  }

  const canonicalAmount = amount.replace(/^0+(?=\d)/, "");
  if (decimals === 0) {
    return canonicalAmount;
  }

  const paddedAmount = canonicalAmount.padStart(decimals + 1, "0");
  const whole = paddedAmount.slice(0, -decimals).replace(/^0+(?=\d)/, "");
  const fraction = paddedAmount.slice(-decimals).replace(/0+$/, "");
  return fraction ? `${whole}.${fraction}` : whole;
}

// decimalToAtomicUnits converts a decimal string without using floating point.
export function decimalToAtomicUnits(
  value: string,
  decimals = supportedAssetDecimals,
): string {
  assertValidDecimals(decimals);
  const normalizedValue = value.trim();
  const match = /^(\d+)(?:\.(\d+))?$/.exec(normalizedValue);
  if (!match || (match[2]?.length ?? 0) > decimals) {
    throw new Error(
      `Enter a valid decimal price with up to ${decimals} decimal places.`,
    );
  }

  const whole = match[1].replace(/^0+(?=\d)/, "");
  const fraction = (match[2] ?? "").padEnd(decimals, "0");
  const atomicAmount = `${whole}${fraction}`.replace(/^0+(?=\d)/, "");
  return atomicAmount || "0";
}

// formatAtomicPrice appends the exact asset to an atomic amount.
export function formatAtomicPrice(
  amount: string,
  asset: string,
  decimals = supportedAssetDecimals,
): string {
  return `${formatAtomicUnits(amount, decimals)} ${asset}`;
}
