export const launchSteps = [
  {
    number: "01",
    title: "Connect",
    description: "Add one scoped AgentPay key to Claude Code, Codex, or another MCP host.",
  },
  {
    number: "02",
    title: "Generate",
    description: "Your coding agent prepares products, checkout, discovery files, and verification.",
  },
  {
    number: "03",
    title: "Verify",
    description: "AgentPay tests signatures, payment gates, metadata, replay safety, and fulfillment.",
  },
  {
    number: "04",
    title: "Sell",
    description: "Approve the changes and accept purchases from people and software agents.",
  },
] as const;

export const productCapabilities = [
  {
    title: "Launch Rail",
    eyebrow: "One guided setup",
    description:
      "Connect your repository once. AgentPay gives your coding agent the exact plan, packages, tests, and publishing checks for your stack.",
    icon: "code",
  },
  {
    title: "Agent Checkout",
    eyebrow: "One commerce path",
    description:
      "Agents and browser wallets use the same frozen price, approval rules, payment verification, and fulfillment endpoint.",
    icon: "agent",
  },
  {
    title: "Discovery Mesh",
    eyebrow: "Be understood everywhere",
    description:
      "Generate truthful metadata, structured data, sitemaps, llms.txt, and machine-readable product manifests from published routes.",
    icon: "globe",
  },
  {
    title: "Proof Stream",
    eyebrow: "Every step accounted for",
    description:
      "Create a signed evidence trail for payment, forwarding, fulfillment, receipts, and deterministic dispute review.",
    icon: "proof",
  },
  {
    title: "Revenue Lens",
    eyebrow: "Know what sold",
    description:
      "Track verified and fulfilled sales by product, asset, network, status, and day without double counting.",
    icon: "analytics",
  },
  {
    title: "Trust Gate",
    eyebrow: "Safety before execution",
    description:
      "Require approval, validate payment, block replay and SSRF, then call the seller exactly once.",
    icon: "security",
  },
] as const;

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
      "The purchase remains payment-required and your fulfillment route is not called. AgentPay is designed to provide a browser-wallet handoff, while card checkout remains a later payment rail.",
  },
  {
    question: "Will AgentPay put my products at the top of search?",
    answer:
      "AgentPay improves technical SEO, answer-engine discovery, metadata consistency, and crawlability. It does not guarantee search ranking, traffic, or sales because external search and agent systems control placement.",
  },
] as const;
