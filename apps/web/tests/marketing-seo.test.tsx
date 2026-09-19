import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import HomePage, { metadata as homeMetadata } from "@/app/page";
import { GET as getLlmsText } from "@/app/llms.txt/route";
import { alt as socialImageAlt, size as socialImageSize } from "@/app/opengraph-image";
import manifest from "@/app/manifest";
import robots from "@/app/robots";
import sitemap from "@/app/sitemap";
import { metadata as rootMetadata } from "@/app/layout";
import {
  agentCheckoutDemoMetadata as demoMetadata,
  sellerDocsMetadata as docsMetadata,
} from "@/features/marketing/seo";

const canonicalOrigin = "https://agentpay.prathamranka.in";

describe("AgentPay marketing discovery", () => {
  it("publishes canonical search and social metadata for the homepage", () => {
    expect(rootMetadata.metadataBase?.toString()).toBe(`${canonicalOrigin}/`);
    expect(rootMetadata.manifest).toBe("/manifest.webmanifest");
    expect(homeMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/`,
    });
    expect(homeMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/`,
      siteName: "AgentPay",
      images: [
        expect.objectContaining({
          url: `${canonicalOrigin}/opengraph-image`,
          width: 1200,
          height: 630,
        }),
      ],
    });
    expect(homeMetadata.twitter).toMatchObject({
      card: "summary_large_image",
      images: [`${canonicalOrigin}/opengraph-image`],
    });
    expect(homeMetadata.keywords).toEqual(
      expect.arrayContaining([
        "x402 payments",
        "API monetization",
        "MCP integration",
        "AI agent commerce",
      ]),
    );
    expect(rootMetadata.authors).toEqual([
      {
        name: "Pratham Ranka",
        url: "https://www.linkedin.com/in/prathamranka06/",
      },
      { name: "Ayush Garg", url: "https://github.com/gargayush1911" },
    ]);
    expect(rootMetadata.icons).toMatchObject({ shortcut: "/favicon.ico" });
    expect(socialImageSize).toEqual({ width: 1200, height: 630 });
    expect(socialImageAlt).toMatch(/AgentPay.*API storefronts/i);
  });

  it("publishes canonical social metadata for public supporting pages", () => {
    expect(docsMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/docs`,
    });
    expect(docsMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/docs`,
      siteName: "AgentPay",
    });
    expect(docsMetadata.twitter).toMatchObject({
      card: "summary_large_image",
    });

    expect(demoMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/demo/agent-checkout`,
    });
    expect(demoMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/demo/agent-checkout`,
      siteName: "AgentPay",
    });
    expect(demoMetadata.twitter).toMatchObject({
      card: "summary_large_image",
    });
  });

  it("keeps private workflow routes out of crawl while exposing discovery", () => {
    expect(robots()).toEqual({
      rules: {
        userAgent: "*",
        allow: "/",
        disallow: [
          "/api/",
          "/dashboard",
          "/purchases",
          "/sign-in",
          "/sign-up",
          "/recover",
          "/verify",
          "/buyer",
        ],
      },
      host: canonicalOrigin,
      sitemap: `${canonicalOrigin}/sitemap.xml`,
    });
  });

  it("lists only public, stable marketing routes in the root sitemap", () => {
    expect(sitemap().map(({ url }) => url)).toEqual([
      `${canonicalOrigin}/`,
      `${canonicalOrigin}/docs`,
      `${canonicalOrigin}/developers`,
      `${canonicalOrigin}/demo/agent-checkout`,
      `${canonicalOrigin}/security`,
      `${canonicalOrigin}/privacy`,
      `${canonicalOrigin}/terms`,
    ]);
  });

  it("publishes a dark AgentPay web manifest with local brand assets", () => {
    expect(manifest()).toMatchObject({
      id: "/",
      name: "AgentPay",
      short_name: "AgentPay",
      start_url: "/",
      display: "standalone",
      background_color: "#050505",
      theme_color: "#050505",
    });
    expect(manifest().icons).toEqual([
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
    ]);
  });

  it("serves concise machine-readable product facts and limitations", async () => {
    const response = getLlmsText();
    const body = await response.text();

    expect(response.headers.get("content-type")).toContain("text/plain");
    expect(body).toContain("# AgentPay");
    expect(body).toContain(`${canonicalOrigin}/`);
    expect(body).toContain("21 maintained stacks");
    expect(body).toContain("x402 testnet");
    expect(body).toContain("does not guarantee search ranking");
    expect(body).toContain("https://github.com/PrathamRanka/fourGeez");
    expect(body).toContain("Pratham Ranka and Ayush Garg");
    expect(body).not.toContain("production-ready");
  });

  it("renders one truthful H1 with WebSite and visible FAQ structured data", () => {
    const { container } = render(<HomePage />);

    expect(container.querySelectorAll("h1")).toHaveLength(1);
    expect(
      screen.getByText(/seller-first x402 commerce · development preview/i),
    ).toBeVisible();
    expect(screen.getByText(/x402 testnet checkout/i)).toBeVisible();
    expect(screen.getByText("Testnet transaction example")).toBeVisible();

    const structuredData = Array.from(
      container.querySelectorAll('script[type="application/ld+json"]'),
    ).map((script) => JSON.parse(script.textContent ?? "{}"));
    expect(structuredData).toContainEqual(
      expect.objectContaining({
        "@context": "https://schema.org",
        "@graph": expect.arrayContaining([
          expect.objectContaining({
            "@type": "WebSite",
            name: "AgentPay",
            url: `${canonicalOrigin}/`,
          }),
          expect.objectContaining({
            "@type": "Organization",
            name: "AgentPay",
          }),
          expect.objectContaining({
            "@type": "WebApplication",
            name: "AgentPay",
          }),
          expect.objectContaining({
            "@type": "SoftwareSourceCode",
            codeRepository: "https://github.com/PrathamRanka/fourGeez",
          }),
        ]),
      }),
    );
    expect(structuredData).toContainEqual(expect.objectContaining({
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: expect.arrayContaining([
        expect.objectContaining({
          "@type": "Question",
          name: "Will AgentPay put my products at the top of search?",
        }),
      ]),
    }));
  });
});
