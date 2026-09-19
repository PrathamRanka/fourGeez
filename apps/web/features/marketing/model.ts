import SiAstro, {
  defaultColor as astroColor,
} from "@icons-pack/react-simple-icons/icons/SiAstro";
import SiDjango from "@icons-pack/react-simple-icons/icons/SiDjango";
import SiDotnet, {
  defaultColor as dotnetColor,
} from "@icons-pack/react-simple-icons/icons/SiDotnet";
import SiExpress from "@icons-pack/react-simple-icons/icons/SiExpress";
import SiFastapi from "@icons-pack/react-simple-icons/icons/SiFastapi";
import SiFastify from "@icons-pack/react-simple-icons/icons/SiFastify";
import SiFlask from "@icons-pack/react-simple-icons/icons/SiFlask";
import SiGin from "@icons-pack/react-simple-icons/icons/SiGin";
import SiGo, {
  defaultColor as goColor,
} from "@icons-pack/react-simple-icons/icons/SiGo";
import SiLaravel from "@icons-pack/react-simple-icons/icons/SiLaravel";
import SiNestjs from "@icons-pack/react-simple-icons/icons/SiNestjs";
import SiNodedotjs, {
  defaultColor as nodeColor,
} from "@icons-pack/react-simple-icons/icons/SiNodedotjs";
import SiNextdotjs from "@icons-pack/react-simple-icons/icons/SiNextdotjs";
import SiNuxt, {
  defaultColor as nuxtColor,
} from "@icons-pack/react-simple-icons/icons/SiNuxt";
import SiOpenjdk from "@icons-pack/react-simple-icons/icons/SiOpenjdk";
import SiPhp, {
  defaultColor as phpColor,
} from "@icons-pack/react-simple-icons/icons/SiPhp";
import SiPython, {
  defaultColor as pythonColor,
} from "@icons-pack/react-simple-icons/icons/SiPython";
import SiReact, {
  defaultColor as reactColor,
} from "@icons-pack/react-simple-icons/icons/SiReact";
import SiRemix from "@icons-pack/react-simple-icons/icons/SiRemix";
import SiRuby, {
  defaultColor as rubyColor,
} from "@icons-pack/react-simple-icons/icons/SiRuby";
import SiRubyonrails from "@icons-pack/react-simple-icons/icons/SiRubyonrails";
import SiSpringboot from "@icons-pack/react-simple-icons/icons/SiSpringboot";
import SiSvelte, {
  defaultColor as svelteColor,
} from "@icons-pack/react-simple-icons/icons/SiSvelte";

export const supportedStacks = [
  { name: "Next.js", icon: SiNextdotjs },
  { name: "React / Vite", icon: SiReact },
  { name: "Remix", icon: SiRemix },
  { name: "Nuxt", icon: SiNuxt },
  { name: "SvelteKit", icon: SiSvelte },
  { name: "Astro", icon: SiAstro },
  { name: "Express", icon: SiExpress },
  { name: "Fastify", icon: SiFastify },
  { name: "NestJS", icon: SiNestjs },
  { name: "Go net/http", icon: SiGo },
  { name: "Gin", icon: SiGin },
  { name: "Echo", icon: SiGo },
  { name: "Fiber", icon: SiGo },
  { name: "FastAPI", icon: SiFastapi },
  { name: "Starlette", icon: SiPython },
  { name: "Flask", icon: SiFlask },
  { name: "Django", icon: SiDjango },
  { name: "ASP.NET Core", icon: SiDotnet },
  { name: "Spring Boot", icon: SiSpringboot },
  { name: "Rails", icon: SiRubyonrails },
  { name: "Laravel", icon: SiLaravel },
] as const;

// The public proof rail groups framework recipes by their owning ecosystem to
// avoid repeating one language/runtime logo for multiple maintained adapters.
export const supportedEcosystems = [
  { name: "Next.js", icon: SiNextdotjs, color: "#ffffff" },
  { name: "React / Vite", icon: SiReact, color: reactColor },
  { name: "Remix", icon: SiRemix, color: "#ffffff" },
  { name: "Nuxt", icon: SiNuxt, color: nuxtColor },
  { name: "SvelteKit", icon: SiSvelte, color: svelteColor },
  { name: "Astro", icon: SiAstro, color: astroColor },
  { name: "Node.js", icon: SiNodedotjs, color: nodeColor },
  { name: "Go", icon: SiGo, color: goColor },
  { name: "Python", icon: SiPython, color: pythonColor },
  { name: ".NET", icon: SiDotnet, color: dotnetColor },
  { name: "Java", icon: SiOpenjdk, color: "#ffffff" },
  { name: "Ruby", icon: SiRuby, color: rubyColor },
  { name: "PHP", icon: SiPhp, color: phpColor },
] as const;

export const supportedAgents = [
  "Claude Code",
  "Codex",
  "Any MCP host",
] as const;

export const launchSignals = [
  {
    eyebrow: "01",
    value: "Keep your product",
    label: "Connect the API and checkout flow you already operate.",
  },
  {
    eyebrow: "02",
    value: "Control every offer",
    label: "Approve routes, prices, and launch changes before they go live.",
  },
  {
    eyebrow: "03",
    value: "Get paid directly",
    label: "Buyer funds settle to your verified wallet.",
  },
] as const;

export const agentPayPlans = [
  {
    name: "Starter",
    audience: "For a focused catalog",
    price: "$6",
    priceLabel: "per month",
    featured: false,
    features: [
      "5 published products",
      "10,000 API requests / month",
      "1,000 MCP operations",
      "30-day evidence retention",
      "Signed webhooks",
    ],
  },
  {
    name: "Growth",
    audience: "For expanding agent revenue",
    price: "$10",
    priceLabel: "per month",
    featured: true,
    features: [
      "50 published products",
      "100,000 API requests / month",
      "10,000 MCP operations",
      "180-day evidence retention",
      "Advanced analytics",
    ],
  },
  {
    name: "Scale",
    audience: "For high-volume platforms",
    price: "$15",
    priceLabel: "per month",
    featured: false,
    features: [
      "500 published products",
      "1,000,000 API requests / month",
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
      "Only exact Base Sepolia USDC is enabled in the current testnet runtime. The purchase remains payment-required and fulfillment is not called when that capability is unavailable. If settlement confirmation is temporarily unavailable after verification, AgentPay retries the same signed payment instead of asking for a second authorization. Card checkout remains deferred.",
  },
  {
    question: "Will AgentPay put my products at the top of search?",
    answer:
      "AgentPay improves technical SEO, answer-engine discovery, metadata consistency, and crawlability. Discovery never authorizes a transaction and does not guarantee search ranking, traffic, or sales.",
  },
  {
    question: "How is AgentPay priced?",
    answer:
      "Launch pricing is $6/month for Starter, $10/month for Growth, and $15/month for Scale. Stripe checkout is not yet enabled in this development preview. Buyer settlement remains separate and goes directly to the seller.",
  },
] as const;
