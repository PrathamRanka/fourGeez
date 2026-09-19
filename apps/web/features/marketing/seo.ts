import type { Metadata, MetadataRoute } from "next";

export const agentPaySiteOrigin = "https://agentpay.prathamranka.in";

export const marketingDescription =
  "AgentPay helps API sellers create agent-ready storefronts with x402 testnet checkout, signed fulfillment, MCP integration, and machine-readable discovery.";

export const socialImage = {
  url: `${agentPaySiteOrigin}/opengraph-image`,
  secureUrl: `${agentPaySiteOrigin}/opengraph-image`,
  width: 1200,
  height: 630,
  type: "image/png",
  alt: "AgentPay — seller-first x402 API storefronts for AI agents",
};

const marketingKeywords = [
  "AI agent commerce",
  "x402 payments",
  "API monetization",
  "API storefront",
  "MCP integration",
  "USDC payments",
  "agent-ready API",
  "seller infrastructure",
] as const;

export const marketingMetadata: Metadata = {
  title: {
    absolute: "AgentPay | Seller-first x402 storefronts for APIs",
  },
  description: marketingDescription,
  keywords: [...marketingKeywords],
  alternates: { canonical: `${agentPaySiteOrigin}/` },
  openGraph: {
    type: "website",
    locale: "en_US",
    siteName: "AgentPay",
    title: "AgentPay | Seller-first x402 storefronts for APIs",
    description: marketingDescription,
    url: `${agentPaySiteOrigin}/`,
    images: [socialImage],
  },
  twitter: {
    card: "summary_large_image",
    title: "AgentPay | Seller-first x402 storefronts for APIs",
    description: marketingDescription,
    images: [socialImage.url],
  },
  robots: { index: true, follow: true },
};

export const marketingStructuredData = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "@id": `${agentPaySiteOrigin}/#organization`,
      name: "AgentPay",
      url: `${agentPaySiteOrigin}/`,
      logo: `${agentPaySiteOrigin}/brand/agentpay-icon-512.png`,
      founder: [
        {
          "@type": "Person",
          name: "Pratham Ranka",
          url: "https://www.linkedin.com/in/prathamranka06/",
        },
        {
          "@type": "Person",
          name: "Ayush Garg",
          url: "https://www.linkedin.com/in/gargayush1911/",
        },
      ],
    },
    {
      "@type": "WebSite",
      "@id": `${agentPaySiteOrigin}/#website`,
      name: "AgentPay",
      url: `${agentPaySiteOrigin}/`,
      description: marketingDescription,
      inLanguage: "en",
      publisher: { "@id": `${agentPaySiteOrigin}/#organization` },
    },
    {
      "@type": "WebApplication",
      "@id": `${agentPaySiteOrigin}/#application`,
      name: "AgentPay",
      url: `${agentPaySiteOrigin}/`,
      applicationCategory: "BusinessApplication",
      operatingSystem: "Web",
      description: marketingDescription,
      featureList: [
        "Seller storefront generation",
        "x402 testnet checkout",
        "MCP-assisted repository integration",
        "Signed fulfillment",
        "Machine-readable product discovery",
      ],
    },
    {
      "@type": "SoftwareSourceCode",
      "@id": `${agentPaySiteOrigin}/#source`,
      name: "AgentPay",
      codeRepository: "https://github.com/PrathamRanka/fourGeez",
      programmingLanguage: ["Go", "TypeScript"],
      copyrightHolder: [
        { "@type": "Person", name: "Pratham Ranka" },
        {
          "@type": "Person",
          name: "Ayush Garg",
          url: "https://www.linkedin.com/in/gargayush1911/",
        },
      ],
    },
  ],
} as const;

export function buildSupportingPageMetadata({
  title,
  description,
  path,
}: {
  title: string;
  description: string;
  path: string;
}): Metadata {
  const canonicalUrl = new URL(path, `${agentPaySiteOrigin}/`).toString();
  return {
    title,
    description,
    alternates: { canonical: canonicalUrl },
    openGraph: {
      type: "website",
      locale: "en_US",
      siteName: "AgentPay",
      title: `${title} | AgentPay`,
      description,
      url: canonicalUrl,
      images: [socialImage],
    },
    twitter: {
      card: "summary_large_image",
      title: `${title} | AgentPay`,
      description,
      images: [socialImage.url],
    },
    robots: { index: true, follow: true },
  };
}

export const sellerDocsMetadata = buildSupportingPageMetadata({
  title: "Seller integration guide",
  description:
    "Connect AgentPay to a supported coding agent, review proposed API storefront changes, and validate the seller integration before publishing.",
  path: "/docs",
});

export const agentCheckoutDemoMetadata = buildSupportingPageMetadata({
  title: "Agent checkout demo",
  description:
    "See a deterministic buyer agent discover a sample offer and enter AgentPay's shared x402 testnet checkout flow.",
  path: "/demo/agent-checkout",
});

export const developersMetadata = buildSupportingPageMetadata({
  title: "Developers",
  description:
    "Meet Pratham Ranka and Ayush Garg, the co-founders building AgentPay's seller-first commerce infrastructure.",
  path: "/developers",
});

export function buildMarketingRobots(): MetadataRoute.Robots {
  return {
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
    host: agentPaySiteOrigin,
    sitemap: `${agentPaySiteOrigin}/sitemap.xml`,
  };
}

const publicMarketingPaths = [
  "/",
  "/docs",
  "/developers",
  "/demo/agent-checkout",
  "/security",
  "/privacy",
  "/terms",
] as const;

export function buildMarketingSitemap(): MetadataRoute.Sitemap {
  return publicMarketingPaths.map((path, index) => ({
    url: new URL(path, `${agentPaySiteOrigin}/`).toString(),
    changeFrequency: path === "/" ? "weekly" : "monthly",
    priority: index === 0 ? 1 : path === "/docs" ? 0.8 : 0.5,
  }));
}

export function buildMarketingManifest(): MetadataRoute.Manifest {
  return {
    id: "/",
    name: "AgentPay",
    short_name: "AgentPay",
    description: marketingDescription,
    start_url: "/",
    display: "standalone",
    background_color: "#050505",
    theme_color: "#050505",
    lang: "en",
    categories: ["business", "developer tools"],
  };
}

export const marketingLlmsText = `# AgentPay

> Seller-first commerce infrastructure for APIs and digitally fulfilled services.

Canonical site: ${agentPaySiteOrigin}/

## Current status

AgentPay is a development preview. Seller workflows are verified locally, AWS deployment and release checks are still in progress, and payment support is limited to mock mode or x402 testnet.

## What AgentPay does

- Prepares a storefront for an existing HTTPS API or digital service.
- Uses Claude Code, Codex, or a generic MCP host to propose reviewable repository changes.
- Keeps publication, pricing, credentials, and deployment subject to explicit seller approval.
- Sends buyer funds directly to the seller's verified wallet; AgentPay does not custody seller revenue in V1.
- Records transaction, fulfillment, evidence, receipt, and dispute facts in one seller control plane.
- Generates canonical metadata, structured data, sitemap entries, llms.txt, and signed storefront manifests for discovery.

## Maintained integrations

AgentPay has 21 maintained stacks: Next.js, React/Vite with a Node API, Remix, Nuxt, SvelteKit, Astro, Express, Fastify, NestJS, Go net/http, Gin, Echo, Fiber, FastAPI, Starlette, Flask, Django, ASP.NET Core, Spring Boot, Rails, and Laravel.

## Public pages

- Home: ${agentPaySiteOrigin}/
- Seller integration guide: ${agentPaySiteOrigin}/docs
- Developers: ${agentPaySiteOrigin}/developers
- Agent checkout demonstration: ${agentPaySiteOrigin}/demo/agent-checkout
- Security: ${agentPaySiteOrigin}/security
- Privacy: ${agentPaySiteOrigin}/privacy
- Terms: ${agentPaySiteOrigin}/terms

## Project and maintainers

- Source repository: https://github.com/PrathamRanka/fourGeez
- Maintainers: Pratham Ranka and Ayush Garg
- Pratham Ranka: https://www.linkedin.com/in/prathamranka06/
- Ayush Garg: https://www.linkedin.com/in/gargayush1911/

## Limits

- Discovery metadata is candidate information and never authorizes a transaction.
- Technical SEO and answer-engine discovery can improve crawlability, but AgentPay does not guarantee search ranking, traffic, conversion, or sales.
- Card checkout, real-funds release validation, and the global marketplace directory are not part of the current release.
`;
