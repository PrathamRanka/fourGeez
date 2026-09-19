import { readFileSync } from "node:fs";
import path from "node:path";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { metadata } from "@/app/layout";
import manifest from "@/app/manifest";
import { AGENTPAY_SYMBOL_PATH } from "@/components/site/brand-symbol";
import { BrandMark } from "@/components/site/brand-mark";

const BRAND_ASSET_DIRECTORY = path.resolve(process.cwd(), "public/brand");

function readPngDimensions(fileName: string) {
  const file = readFileSync(path.join(BRAND_ASSET_DIRECTORY, fileName));

  expect(file.subarray(1, 4).toString("ascii")).toBe("PNG");

  return {
    width: file.readUInt32BE(16),
    height: file.readUInt32BE(20),
  };
}

describe("AgentPay brand assets", () => {
  it("keeps the product mark and exported symbol geometry in sync", () => {
    const { container } = render(<BrandMark compact />);

    expect(container.querySelector("svg")).toHaveAttribute(
      "viewBox",
      "0 0 48 48",
    );
    expect(
      container.querySelector(`path[d="${AGENTPAY_SYMBOL_PATH}"]`),
    ).toBeInTheDocument();
    expect(container.querySelectorAll("path")).toHaveLength(1);
  });

  it("ships a text-free vector master and correctly sized app icons", () => {
    const symbolSource = readFileSync(
      path.join(BRAND_ASSET_DIRECTORY, "agentpay-symbol.svg"),
      "utf8",
    );
    const iconSource = readFileSync(
      path.join(BRAND_ASSET_DIRECTORY, "agentpay-icon.svg"),
      "utf8",
    );
    const faviconSource = readFileSync(
      path.join(BRAND_ASSET_DIRECTORY, "agentpay-favicon.svg"),
      "utf8",
    );

    expect(symbolSource).toContain(AGENTPAY_SYMBOL_PATH);
    expect(symbolSource).toContain('fill="currentColor"');
    expect(symbolSource).toContain('fill-rule="evenodd"');
    expect(symbolSource).not.toContain(
      "M20 3.5 34.3 11.8v16.4L20 36.5 5.7 28.2V11.8L20 3.5Z",
    );
    expect(symbolSource).not.toContain("linearGradient");
    expect(symbolSource).not.toMatch(/<text\b/i);
    expect(iconSource).toContain(AGENTPAY_SYMBOL_PATH);
    expect(iconSource).toContain("#2979ff");
    expect(iconSource).toContain("#ff5aa5");
    expect(iconSource).not.toContain("#ff6d00");
    expect(iconSource).not.toMatch(/<text\b/i);
    expect(faviconSource).toContain("#f7f7f4");
    expect(faviconSource).toContain(AGENTPAY_SYMBOL_PATH);
    expect(faviconSource).not.toContain("linearGradient");
    expect(faviconSource).not.toContain("#2979ff");
    expect(faviconSource).not.toContain("#ff5aa5");
    expect(faviconSource).not.toContain("#ff6d00");
    expect(faviconSource).not.toMatch(/<text\b/i);
    expect(readPngDimensions("agentpay-icon-16.png")).toEqual({
      width: 16,
      height: 16,
    });
    expect(readPngDimensions("agentpay-icon-32.png")).toEqual({
      width: 32,
      height: 32,
    });
    expect(readPngDimensions("agentpay-icon-180.png")).toEqual({
      width: 180,
      height: 180,
    });
    expect(readPngDimensions("agentpay-icon-192.png")).toEqual({
      width: 192,
      height: 192,
    });
    expect(readPngDimensions("agentpay-icon-512.png")).toEqual({
      width: 512,
      height: 512,
    });

    const favicon = readFileSync(
      path.resolve(process.cwd(), "public/favicon.ico"),
    );
    expect(favicon.readUInt16LE(0)).toBe(0);
    expect(favicon.readUInt16LE(2)).toBe(1);
    expect(favicon.readUInt16LE(4)).toBe(2);
  });

  it("publishes browser and install metadata for the same icon family", () => {
    expect(metadata.metadataBase?.toString()).toBe(
      "https://agentpay.prathamranka.in/",
    );
    expect(metadata.manifest).toBe("/manifest.webmanifest");
    expect(metadata.icons).toEqual({
      icon: [
        {
          url: "/brand/agentpay-favicon.svg",
          type: "image/svg+xml",
        },
        {
          url: "/favicon.ico",
          sizes: "16x16 32x32",
          type: "image/x-icon",
        },
        {
          url: "/brand/agentpay-icon-32.png",
          sizes: "32x32",
          type: "image/png",
        },
      ],
      apple: [
        {
          url: "/brand/agentpay-icon-180.png",
          sizes: "180x180",
          type: "image/png",
        },
      ],
    });

    expect(manifest()).toMatchObject({
      name: "AgentPay",
      short_name: "AgentPay",
      theme_color: "#050505",
      background_color: "#050505",
      icons: [
        {
          src: "/brand/agentpay-icon-192.png",
          sizes: "192x192",
          type: "image/png",
          purpose: "any",
        },
        {
          src: "/brand/agentpay-icon-512.png",
          sizes: "512x512",
          type: "image/png",
          purpose: "maskable",
        },
      ],
    });
  });
});
