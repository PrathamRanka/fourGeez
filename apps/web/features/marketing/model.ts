export const supportedStacks = [
  "Next.js",
  "React",
  "Remix",
  "Nuxt",
  "SvelteKit",
  "Astro",
  "Express",
  "NestJS",
  "Go",
  "FastAPI",
  "Django",
  "ASP.NET",
  "Spring Boot",
  "Rails",
  "Laravel",
] as const;

export const supportedAgents = [
  "Claude Code",
  "Codex",
  "Any MCP host",
] as const;

export const launchSignals = [
  { value: "1 prompt", label: "Repository setup" },
  { value: "15+", label: "Maintained stacks" },
  { value: "0", label: "Wallet keys held" },
] as const;

export const agentPayPlans = [
  {
    name: "Starter",
    audience: "For a focused catalog",
    volume: "10K",
    volumeLabel: "API requests / month",
    featured: false,
    features: [
      "5 published products",
      "1,000 MCP operations",
      "30-day evidence retention",
      "Signed webhooks",
    ],
  },
  {
    name: "Growth",
    audience: "For expanding agent revenue",
    volume: "100K",
    volumeLabel: "API requests / month",
    featured: true,
    features: [
      "50 published products",
      "10,000 MCP operations",
      "180-day evidence retention",
      "Advanced analytics",
    ],
  },
  {
    name: "Scale",
    audience: "For high-volume platforms",
    volume: "1M",
    volumeLabel: "API requests / month",
    featured: false,
    features: [
      "500 published products",
      "100,000 MCP operations",
      "10-year evidence retention",
      "Priority support",
    ],
  },
] as const;

export const frequentlyAskedQuestions = [
  {
    question: "Do I need to rebuild my product?",
    answer:
      "No. AgentPay sits in front of your existing HTTPS service. Your coding agent adds a small verification boundary, product discovery files, and the storefront integration while your application continues to fulfill the product.",
  },
  {
    question: "Where does buyer money go?",
    answer:
      "Buyer funds settle directly to your verified wallet for the configured asset and network. AgentPay records and verifies the transaction but does not custody your sales revenue in V1.",
  },
  {
    question: "What if an agent cannot pay with x402?",
    answer:
      "The purchase remains payment-required and your fulfillment route is not called. AgentPay provides a browser-wallet handoff, while card checkout remains a later payment rail.",
  },
  {
    question: "Will AgentPay put my products at the top of search?",
    answer:
      "AgentPay improves technical SEO, answer-engine discovery, metadata consistency, and crawlability. Discovery never authorizes a transaction and does not guarantee search ranking, traffic, or sales.",
  },
  {
    question: "How is AgentPay priced?",
    answer:
      "AgentPay is seller-funded through a monthly software plan, with optional metered fees for successful transactions and higher tiers for approvals, evidence retention, analytics, limits, and support. Buyer settlement remains separate and goes directly to the seller.",
  },
] as const;
