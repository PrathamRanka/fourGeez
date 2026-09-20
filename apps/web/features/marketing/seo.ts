import type { Metadata, MetadataRoute } from "next";

export const agentPaySiteOrigin = "https://agentpay.prathamranka.in";

export const marketingDescription =
  "AgentPay helps API sellers create agent-ready storefronts with x402 testnet checkout, signed fulfillment, MCP integration, and machine-readable discovery.";

export const globalEnglishOpenGraphLocales = [
  "en_GB",
  "en_IN",
  "en_ZA",
  "en_AU",
  "en_GY",
] as const;

const globalServiceAreas = [
  "Africa",
  "Asia",
  "Europe",
  "North America",
  "South America",
  "Oceania",
] as const;

function buildLanguageAlternates(canonicalUrl: string) {
  return {
    canonical: canonicalUrl,
    languages: {
      en: canonicalUrl,
      "x-default": canonicalUrl,
    },
  };
}

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
  alternates: buildLanguageAlternates(`${agentPaySiteOrigin}/`),
  openGraph: {
    type: "website",
    locale: "en_US",
    alternateLocale: [...globalEnglishOpenGraphLocales],
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
      alternateName: "AgentPay API Commerce",
      url: `${agentPaySiteOrigin}/`,
      logo: `${agentPaySiteOrigin}/brand/agentpay-icon-512.png`,
      sameAs: [
        "https://github.com/PrathamRanka/fourGeez",
        "https://www.linkedin.com/in/prathamranka06/",
        "https://www.linkedin.com/in/gargayush1911/",
      ],
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
        "Coding-agent repository integration with AgentPay MCP guidance",
        "Signed fulfillment",
        "Machine-readable product discovery",
      ],
    },
    {
      "@type": "Service",
      "@id": `${agentPaySiteOrigin}/#service`,
      name: "AgentPay API commerce infrastructure",
      serviceType: "Seller-first commerce infrastructure for APIs",
      provider: { "@id": `${agentPaySiteOrigin}/#organization` },
      url: `${agentPaySiteOrigin}/`,
      description: marketingDescription,
      availableLanguage: "English",
      areaServed: globalServiceAreas.map((name) => ({
        "@type": "Place",
        name,
      })),
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
    alternates: buildLanguageAlternates(canonicalUrl),
    openGraph: {
      type: "website",
      locale: "en_US",
      alternateLocale: [...globalEnglishOpenGraphLocales],
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
  title: "External buyer compatibility demo",
  description:
    "See a deterministic simulation of an external buyer client enter AgentPay's shared x402 testnet checkout flow. AgentPay does not operate a V1 buyer agent.",
  path: "/demo/agent-checkout",
});

export const developersMetadata = buildSupportingPageMetadata({
  title: "Developers",
  description:
    "Meet Pratham Ranka and Ayush Garg, the co-founders building AgentPay's seller-first commerce infrastructure.",
  path: "/developers",
});

export const contactMetadata = buildSupportingPageMetadata({
  title: "Contact",
  description:
    "Contact AgentPay co-founders Pratham Ranka and Ayush Garg about seller onboarding, integrations, and launch access.",
  path: "/contact",
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
  "/contact",
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
- The seller's coding agent edits the repository using AgentPay MCP analysis and guidance.
- AgentPay MCP performs bounded configuration, verification, and seller-confirmed cloud mutations; it does not write repository files.
- Keeps publication, pricing, credentials, and deployment subject to explicit seller approval.
- Sends buyer funds directly to the seller's verified wallet; AgentPay does not custody seller revenue in V1.
- Records transaction, fulfillment, evidence, receipt, and dispute facts in one seller control plane.
- Guides the coding agent in generating canonical metadata, structured data, sitemap entries, llms.txt, and storefront manifests; AgentPay cloud signs and publishes authoritative discovery.

## Maintained integrations

AgentPay has 21 maintained stacks: Next.js, React/Vite with a Node API, Remix, Nuxt, SvelteKit, Astro, Express, Fastify, NestJS, Go net/http, Gin, Echo, Fiber, FastAPI, Starlette, Flask, Django, ASP.NET Core, Spring Boot, Rails, and Laravel.

## Global scope

The English-language public website and machine-readable discovery are available worldwide across Africa, Asia, Europe, North America, South America, and Oceania. Current checkout support remains limited to mock mode or exact Base Sepolia USDC through x402 testnet.

## Authoritative machine-readable entry points

- Agent overview: ${agentPaySiteOrigin}/llms.txt
- Platform capability manifest: ${agentPaySiteOrigin}/api/backend/.well-known/agentpay
- Runtime payment capabilities: ${agentPaySiteOrigin}/api/backend/v1/payment-capabilities
- Public product directory: ${agentPaySiteOrigin}/api/backend/v1/discovery/products
- Seller integration guide: ${agentPaySiteOrigin}/docs

Agents should treat discovery as candidate information, inspect the signed product contract, and rely on the current AgentPay cloud response for purchase authorization.

## Public pages

- Home: ${agentPaySiteOrigin}/
- Seller integration guide: ${agentPaySiteOrigin}/docs
- Developers: ${agentPaySiteOrigin}/developers
- Contact: ${agentPaySiteOrigin}/contact
- External buyer compatibility demonstration: ${agentPaySiteOrigin}/demo/agent-checkout
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
- AgentPay does not operate a V1 buyer agent, A2A execution service, negotiation runtime, or marketplace ranking system.
- Technical SEO and answer-engine discovery can improve crawlability, but AgentPay does not guarantee search ranking, traffic, conversion, or sales.
- Card checkout, real-funds release validation, and the global marketplace directory are not part of the current release.
`;
