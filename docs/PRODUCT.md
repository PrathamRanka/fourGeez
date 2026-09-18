# Product contract

Status: **Locked for the AgentPay MVP direction**.

## Product promise

AgentPay turns an existing API or digital service into a storefront that can
sell to both people and software agents. A seller connects AgentPay to a
supported coding agent, approves the proposed repository changes, and publishes
products without manually implementing payment, approval, evidence, or agent
discovery protocols.

AgentPay is the commerce gateway and control plane. The seller continues to own
and operate the upstream service that fulfills each product.

## Public product experience

AgentPay includes a complete public website for ordinary visitors before they
enter a seller or buyer workflow. The site explains the product without
requiring protocol knowledge, demonstrates the agent purchase path, exposes
supported stacks and payment behavior, answers trust and pricing questions, and
provides clear sign-up and sign-in entry points.

Branded feature names may make the product memorable, but every name must be
paired with a plain-language explanation. The initial vocabulary is:

- **Launch Rail** — the guided API-key and coding-agent setup flow;
- **Agent Checkout** — the shared purchase flow for agents and browser wallets;
- **Discovery Mesh** — storefront metadata, manifest, sitemap, and `llms.txt`;
- **Proof Stream** — transaction, payment, fulfillment, and evidence history;
- **Revenue Lens** — asset- and network-separated seller analytics; and
- **Trust Gate** — approval, replay protection, sandbox, and publication checks.

The public site may use polished demonstrations and restrained motion, but must
remain fast, accessible, usable without animation, and truthful. It must never
claim guaranteed search ranking, guaranteed revenue, automatic production
deployment without consent, or support for a payment rail that is not enabled.

## Initial product scope

The first commercial scope is API-backed and digitally fulfilled products:

- paid API calls;
- generated reports and research;
- datasets and digital downloads;
- bounded SaaS actions; and
- other responses that can be delivered synchronously by an HTTPS endpoint.

A published `paidRoute` is the V1 product record. Physical goods, shipping,
inventory, tax calculation, and physical returns are outside the initial scope.

## Seller experience

The intended onboarding flow is:

1. The seller creates an AgentPay seller account and storefront.
2. AgentPay issues a project-scoped integration credential.
3. The seller connects the AgentPay MCP server to a supported coding agent such
   as Claude Code or Codex.
4. The coding agent inspects the seller repository and proposes sellable routes,
   storefront pages, verification middleware, technical SEO/AEO improvements,
   agent-discovery metadata, configuration, and tests.
5. The seller reviews the proposed prices, routes, generated code, and deployment
   changes.
6. Only after explicit confirmation may the integration publish products or
   change AgentPay configuration.
7. Buyers use the hosted storefront or machine-readable AgentPay endpoints.
8. Every successful sale appears in one seller dashboard regardless of channel.

A representative prompt is:

```text
Connect this project to AgentPay. Identify sellable API routes, propose products
and prices, install AgentPay request verification, generate the storefront and
stack-native technical SEO/AEO metadata, run the integration tests, and prepare
the changes for my approval.
```

The coding agent may prepare code and configuration automatically. It must not
invent prices, publish products, rotate credentials, or deploy production
changes without explicit seller confirmation.

SEO/AEO generation improves crawlability and machine discovery but never
guarantees placement, traffic, conversion, or ranking in a search engine or AI
answer. Generated changes must avoid keyword stuffing, hidden content, doorway
pages, fabricated reviews, fabricated claims, and schema markup that is not
supported by visible page content.

## Seller integration surface

V1 does not require a large seller SDK. AgentPay provides:

- a seller control API and minimal dashboard;
- a remote MCP server with scoped tools, resources, and setup prompts;
- coding-agent setup instructions for Claude Code, Codex, and generic MCP hosts;
- maintained request-verification middleware or small packages for supported
  server frameworks;
- framework templates and copyable examples;
- a sandbox validation command; and
- generated public storefront, manifest, `llms.txt`, sitemap, structured data,
  canonical metadata, and paid URLs.

The seller exposes an HTTPS fulfillment endpoint. AgentPay verifies payment and
approval, claims the transaction exactly once, and forwards a signed request to
that endpoint. Seller code verifies the AgentPay signature and returns the
digital result.

## Buyer channels

### Agent channel

Agents discover products through the storefront manifest or `llms.txt`, create
an immutable purchase intent, satisfy approval when required, pay through the
x402 adapter, and call the AgentPay paid URL.

### Browser channel

People browse a seller-branded storefront, choose the same published products,
and may complete an x402-compatible wallet payment through browser instructions
or a supported wallet flow. Agent and browser purchases enter the same intent,
transaction, evidence, receipt, fulfillment, and dispute pipeline. Card checkout
is deferred and is not required for the agent-first release.

## Unified commerce rule

Browser and agent purchases share the authoritative seller quote and the same
purchase-intent, approval, transaction, fulfillment, evidence, and dispute
rules. Channel and payment-rail metadata may differ, but neither channel may
bypass domain validation.

## Seller payment and reporting

Each seller configures and proves control of an asset-and-network-specific
payment destination. AgentPay never requests or stores the private key. Route
prices reference an active seller payment destination rather than treating an
unverified address as trusted configuration.

AgentPay records payment and transaction facts centrally as requests pass
through the gateway. The coding agent is required for repository setup, not for
ongoing sales reporting. Seller dashboards derive gross verified payments,
fulfilled sales, failures, disputes, route performance, and usage from AgentPay
records. Amounts are always grouped by asset and network and are never summed
across unlike currencies.

Sellers may configure signed webhooks for payment, fulfillment, and dispute
events. Webhook delivery is retried and audited, but dashboard records remain
authoritative when a seller endpoint is unavailable.

## Revenue model

The initial revenue model is seller-funded software and usage billing:

- a monthly seller subscription;
- optional metered fees based on successful transactions; and
- higher tiers for approvals, evidence retention, limits, analytics, and support.

Buyer funds continue directly to the seller under the x402 flow. AgentPay does
not custody or redistribute seller funds in V1. AgentPay subscriptions and
metered usage are accounted for separately from buyer settlement and may be
invoiced through a future billing adapter. Card settlement, platform fees,
refunds, tax responsibility, and merchant-of-record status must be explicitly
decided before enabling production card payments.

## Definition of a launch-ready seller

A seller is ready to publish only when:

- authentication and project scope are valid;
- at least one asset-and-network-specific payment destination is verified;
- every product maps to an enabled, validated paid route;
- the upstream target passes SSRF and reachability checks;
- request-signature verification passes the sandbox test;
- prices and payout configuration were explicitly approved by the seller;
- no secret is present in committed files or browser bundles;
- browser and agent storefront representations agree;
- generated metadata, sitemap, structured data, `llms.txt`, and manifest pass
  deterministic consistency checks; and
- a sandbox purchase reaches the intended fulfillment endpoint exactly once.
