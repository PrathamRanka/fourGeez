import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import HomePage, { metadata as homeMetadata } from "@/app/page";
import { GET as getLlmsText } from "@/app/llms.txt/route";
import {
  alt as socialImageAlt,
  size as socialImageSize,
} from "@/app/opengraph-image";
import manifest from "@/app/manifest";
import robots from "@/app/robots";
import sitemap from "@/app/sitemap";
import { metadata as rootMetadata } from "@/app/layout";
import {
  agentCheckoutDemoMetadata as demoMetadata,
  globalEnglishOpenGraphLocales,
  sellerDocsMetadata as docsMetadata,
} from "@/features/marketing/seo";

const canonicalOrigin = "https://agentpay.prathamranka.in";

describe("AgentPay marketing discovery", () => {
  it("publishes canonical search and social metadata for the homepage", () => {
    expect(rootMetadata.metadataBase?.toString()).toBe(`${canonicalOrigin}/`);
    expect(rootMetadata.manifest).toBe("/manifest.webmanifest");
    expect(homeMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/`,
      languages: {
        en: `${canonicalOrigin}/`,
        "x-default": `${canonicalOrigin}/`,
      },
    });
    expect(homeMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/`,
      siteName: "AgentPay",
      alternateLocale: globalEnglishOpenGraphLocales,
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
      {
        name: "Ayush Garg",
        url: "https://www.linkedin.com/in/gargayush1911/",
      },
    ]);
    expect(rootMetadata.icons).toMatchObject({ shortcut: "/favicon.ico" });
    expect(rootMetadata.openGraph).toMatchObject({
      siteName: "AgentPay",
      title: "AgentPay | Seller-first x402 storefronts for APIs",
      description: expect.stringContaining("API sellers"),
      images: [expect.objectContaining({ width: 1200, height: 630 })],
    });
    expect(rootMetadata.twitter).toMatchObject({
      card: "summary_large_image",
      images: [`${canonicalOrigin}/opengraph-image`],
    });
    expect(socialImageSize).toEqual({ width: 1200, height: 630 });
    expect(socialImageAlt).toMatch(/AgentPay.*API storefronts/i);
  });

  it("publishes canonical social metadata for public supporting pages", () => {
    expect(docsMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/docs`,
      languages: {
        en: `${canonicalOrigin}/docs`,
        "x-default": `${canonicalOrigin}/docs`,
      },
    });
    expect(docsMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/docs`,
      siteName: "AgentPay",
      alternateLocale: globalEnglishOpenGraphLocales,
    });
    expect(docsMetadata.twitter).toMatchObject({
      card: "summary_large_image",
    });

    expect(demoMetadata.alternates).toEqual({
      canonical: `${canonicalOrigin}/demo/agent-checkout`,
      languages: {
        en: `${canonicalOrigin}/demo/agent-checkout`,
        "x-default": `${canonicalOrigin}/demo/agent-checkout`,
      },
    });
    expect(demoMetadata.openGraph).toMatchObject({
      type: "website",
      url: `${canonicalOrigin}/demo/agent-checkout`,
      siteName: "AgentPay",
      alternateLocale: globalEnglishOpenGraphLocales,
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
      `${canonicalOrigin}/contact`,
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
    expect(body).toContain("## Authoritative machine-readable entry points");
    expect(body).toContain(
      `${canonicalOrigin}/api/backend/.well-known/agentpay`,
    );
    expect(body).toContain(
      `${canonicalOrigin}/api/backend/v1/payment-capabilities`,
    );
    expect(body).toContain(
      "Africa, Asia, Europe, North America, South America, and Oceania",
    );
    expect(body).toContain("x402 testnet");
    expect(body).toContain("does not guarantee search ranking");
    expect(body).toContain(
      "The seller's coding agent edits the repository using AgentPay MCP analysis and guidance.",
    );
    expect(body).toContain(
      "AgentPay MCP performs bounded configuration, verification, and seller-confirmed cloud mutations; it does not write repository files.",
    );
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
            alternateName: "AgentPay API Commerce",
            sameAs: expect.arrayContaining([
              "https://github.com/PrathamRanka/fourGeez",
            ]),
          }),
          expect.objectContaining({
            "@type": "WebApplication",
            name: "AgentPay",
            featureList: expect.arrayContaining([
              "Coding-agent repository integration with AgentPay MCP guidance",
            ]),
          }),
          expect.objectContaining({
            "@type": "Service",
            name: "AgentPay API commerce infrastructure",
            areaServed: expect.arrayContaining([
              expect.objectContaining({ name: "Africa" }),
              expect.objectContaining({ name: "Asia" }),
              expect.objectContaining({ name: "Europe" }),
              expect.objectContaining({ name: "North America" }),
              expect.objectContaining({ name: "South America" }),
              expect.objectContaining({ name: "Oceania" }),
            ]),
          }),
          expect.objectContaining({
            "@type": "SoftwareSourceCode",
            codeRepository: "https://github.com/PrathamRanka/fourGeez",
          }),
        ]),
      }),
    );
    expect(structuredData).toContainEqual(
      expect.objectContaining({
        "@context": "https://schema.org",
        "@type": "FAQPage",
        mainEntity: expect.arrayContaining([
          expect.objectContaining({
            "@type": "Question",
            name: "Will AgentPay put my products at the top of search?",
          }),
        ]),
      }),
    );
  });
});
