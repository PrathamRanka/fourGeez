export type MerchantHeaders = Record<
  string,
  string | readonly string[] | undefined
>;

export function normalizeHeaders(
  headers: MerchantHeaders,
): Record<string, string> {
  const normalized: Record<string, string> = {};
  for (const [name, value] of Object.entries(headers)) {
    if (typeof value === "string") {
      normalized[name.toLowerCase()] = value;
    } else if (Array.isArray(value) && value.length > 0) {
      normalized[name.toLowerCase()] = value.join(", ");
    }
  }
  return normalized;
}
