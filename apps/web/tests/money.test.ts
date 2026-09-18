import { describe, expect, it } from "vitest";
import { decimalToAtomicUnits, formatAtomicUnits } from "@/lib/money";

describe("decimal and atomic money conversion", () => {
  it.each([
    ["35", "35000000"],
    ["35.125", "35125000"],
    ["0.000001", "1"],
    ["0007.500000", "7500000"],
  ])("converts %s exactly without floating point", (decimal, atomic) => {
    expect(decimalToAtomicUnits(decimal)).toBe(atomic);
    expect(formatAtomicUnits(atomic)).toBe(
      decimal.replace(/^0+(?=\d)/, "").replace(/\.?0+$/, ""),
    );
  });

  it.each(["", "-1", "1e3", "1.0000001", ".5", "1."])(
    "rejects invalid decimal price %s",
    (value) => {
      expect(() => decimalToAtomicUnits(value)).toThrow("decimal price");
    },
  );
});
