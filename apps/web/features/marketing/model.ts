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

export const productFacts = [
  { value: "1 prompt", label: "to prepare the integration" },
  { value: "15+ stacks", label: "with maintained setup recipes" },
  { value: "0 private keys", label: "stored by AgentPay" },
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
      "AgentPay improves technical SEO, answer-engine discovery, metadata consistency, and crawlability. It does not guarantee search ranking, traffic, or sales because external search and agent systems control placement.",
  },
  {
    question: "How is AgentPay priced?",
    answer:
      "AgentPay is seller-funded through a monthly software plan, with optional metered fees for successful transactions and higher tiers for approvals, evidence retention, analytics, limits, and support. Buyer settlement remains separate and goes directly to the seller.",
  },
] as const;
