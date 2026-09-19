# Product contract

Status: **Locked for the AgentPay MVP direction**.

Implementation status: the M0–M7 development preview and the fourteen-task
M7.1 Lean V1 launch core are implemented and verified locally. M8 deployment
and M9 deployed release gates remain incomplete, so the product is not yet a
production-ready paid service. Current payment support is mock or x402 testnet
only.

## Product promise

AgentPay turns an existing API or digital service into a storefront that can
sell to both people and software agents. A seller connects AgentPay to a
supported coding agent, approves the proposed repository changes, and publishes
products without manually implementing payment, evidence, or agent
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
- **Trust Gate** — wallet consent, replay protection, sandbox, and publication checks.

The public site may use polished demonstrations and restrained motion, but must
remain fast, accessible, usable without animation, and truthful. It must never
claim guaranteed search ranking, guaranteed revenue, automatic production
deployment without consent, or support for a payment rail that is not enabled.

## Product surfaces and route meaning

The V1 information architecture has four distinct surfaces:

- the public AgentPay website explains the platform before account creation;
- the authenticated seller dashboard owns onboarding, products, storefront
  publication, transactions, evidence, billing, and operations;
- `/store/{sellerSlug}` is one seller's public storefront containing all of
  that seller's published products; and
- `/store/{sellerSlug}/products/{productSlug}` is the canonical public page for
  one product.

Human buyers purchase from storefront product pages and do not require a
general AgentPay buyer account in V1. Software agents discover products through
the central AgentPay directory, signed storefront manifests, `llms.txt`,
buyer-facing API resources, and paid URLs. The seller-authenticated coding-agent
MCP is a separate integration surface and is not buyer discovery. The
interactive agent demonstration lives at `/demo/agent-checkout`; it explains
and exercises the agent channel but is not the authoritative discovery registry
or a separate commerce pipeline.

Internal identifiers such as `sellerId` and `routeId` are never used as public
navigation labels. The current route-ID product URL remains a compatibility
path only until LCH-003 and LCH-004 introduce the product-slug wire contract and
redirect behavior.

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
2. The seller completes the required profile, enters and verifies a payout
   address for a platform-supported testnet asset/network, and configures an
   HTTPS service origin. The dashboard explains and links every missing
   prerequisite; it does not expose MCP setup before eligibility.
3. AgentPay issues a project-scoped integration credential once and displays
   the raw project key only in that creation response.
4. The seller connects the AgentPay MCP server to a supported coding agent such
   as Claude Code or Codex.
5. The coding agent inspects bounded committed manifests and OpenAPI, detects a
   maintained stack, proposes sellable routes and SEO/AEO changes, installs
   verification middleware, and generates tests. Price and payout fields remain
   explicit seller inputs; the agent never invents or changes them.
6. The seller reviews the prices, routes, generated code, and deployment
   changes.
7. Only after the authenticated seller dashboard issues a short-lived,
   one-time confirmation grant for the exact reviewed change may the cloud MCP
   publish products or change AgentPay configuration. A coding agent cannot
   confirm its own proposal.
8. Buyers use the hosted storefront or machine-readable AgentPay endpoints.
9. Every successful sale appears in one seller dashboard regardless of channel.

A representative prompt is:

```text
Connect this project to AgentPay. Identify sellable API routes, propose product
names and truthful SEO/AEO changes, install AgentPay request verification,
generate focused tests, and prepare the changes for my approval. Ask me for
every exact price and payout destination; never invent either value.
```

The coding agent may prepare code and configuration automatically. It must not
invent prices or payout addresses, change either commercial value, publish
products, rotate credentials, or deploy production changes without explicit
seller confirmation.

Before step 3, the dashboard renders the server-authoritative prerequisite
checklist and links each incomplete item to its completion surface. It does not
render key creation, connector commands, host configuration, or an MCP setup
prompt while any required prerequisite is incomplete. After eligibility, the
dashboard distinguishes the reveal-once project credential from actual
connector authorization, reports credential/connector states (`disconnected`,
`connected`, `expired`, or `revoked`), shows Windows PowerShell preflight and
host-specific configuration, and provides actionable retry guidance. A
published product displays its full canonical storefront URL.

SEO/AEO generation improves crawlability and machine discovery but never
guarantees placement, traffic, conversion, or ranking in a search engine or AI
answer. Generated changes must avoid keyword stuffing, hidden content, doorway
pages, fabricated reviews, fabricated claims, and schema markup that is not
supported by visible page content.

## Seller integration surface

V1 does not require a large seller SDK. AgentPay provides:

- a seller control API and minimal dashboard;
- a remote MCP server with scoped tools, resources, and setup prompts;
- a required AgentPay local connector that exchanges project keys for
  short-lived cloud MCP capabilities; direct remote OAuth is deferred;
- coding-agent setup instructions for Claude Code, Codex, and generic MCP hosts;
- maintained request-verification middleware or small packages for supported
  server frameworks;
- framework templates and copyable examples;
- a sandbox validation command; and
- generated public storefront, manifest, `llms.txt`, sitemap, structured data,
  canonical metadata, and paid URLs.

The seller exposes an HTTPS fulfillment endpoint. AgentPay verifies payment,
claims the transaction exactly once, and forwards a signed request to
that endpoint. Seller code verifies the AgentPay signature and returns the
digital result.

Seller-hosted integration code is not part of AgentPay's trust boundary. It may
be modified or forked, but it receives no private signing material, payment
verification authority, publication authority, subscription authority, or
official transaction state. The official MCP endpoint and every commercial
decision remain in AgentPay's cloud. A project key only bootstraps a short-lived
MCP capability; it is not a transaction credential.

## Buyer channels

### Agent channel

Agents discover products through the storefront manifest or `llms.txt`, create
an immutable purchase intent, enforce the buyer-provided maximum, pay through
the x402 adapter, and call the AgentPay paid URL.

Discovery is candidate information only. Even an authentic, unexpired manifest
does not authorize a purchase; the cloud rechecks current seller entitlement,
route publication, destination, quote, payment, and replay state at
the relevant transaction checkpoints.

### Browser channel

People browse a seller-branded storefront, choose the same published products,
and may complete an x402-compatible wallet payment through browser instructions
or a supported wallet flow. Agent and browser purchases enter the same intent,
transaction, evidence, receipt, fulfillment, and dispute pipeline. Card checkout
is deferred and is not required for the agent-first release.

The public storefront is the browser buyer experience. A separate `/buyer`
account area is not part of V1. Agent-specific interaction is demonstrated at
`/demo/agent-checkout` and uses the same authoritative commerce services.
The current M7 development runtime still requires an agent credential for
intent and paid-route operations. The locked M7.1 contract replaces that gap
with a durable opaque browser purchase grant: commerce authority expires within
ten minutes, receipt/dispute access survives reloads for 30 days after the
terminal outcome or a timely dispute's resolution,
and a finalized payer can recover read/remediation access with wallet proof.
LCH-013 and LCH-014 must implement and verify that target before browser
checkout is represented as complete.

## Unified commerce rule

Browser and agent purchases share the authoritative seller quote and the same
purchase-intent, transaction, fulfillment, evidence, and dispute
rules. Channel and payment-rail metadata may differ, but neither channel may
bypass domain validation.

Lean V1 has no buyer-side multi-person approval runtime. The seller-approved
fixed quote is authoritative, `maximumAmount` is the buyer safety ceiling, and
the buyer wallet authorization/signature is payment consent. The historical M2
approval domain remains in the repository for compatibility and future work,
but its REST routes, WebSocket channel, `428 approval_required` branch, approval
tokens, and approval UI are disabled and excluded from Lean V1. Seller
confirmation for publication, price changes, credentials, and deployment is a
separate control-plane safeguard and remains required.

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

When deterministic dispute rules recommend a refund, the authenticated owning
seller may record one externally completed full refund through the idempotent
refund-record endpoint. AgentPay validates the finalized transaction and exact
amount, asset, and network, then stores the seller-supplied reference as an
append-only audit fact. AgentPay does not execute, custody, or guarantee the
refund in Lean V1.

## Revenue model

The intended revenue model is seller-funded software and usage billing:

- a monthly seller subscription;
- optional metered fees based on successful transactions; and
- higher tiers for evidence retention, limits, analytics, and support.

Buyer funds continue directly to the seller under the x402 flow. AgentPay does
not custody or redistribute seller funds in V1. AgentPay subscriptions and
metered usage are accounted for separately from buyer settlement and may be
invoiced through the Stripe Billing adapter. Paid network participation ends
exactly at `accessEndsAt`; the fixed 72-hour grace period is billing-recovery
and historical-read-only access, not MCP, discovery, publication, or commerce
authority. Card settlement, platform fees,
refunds, tax responsibility, and merchant-of-record status must be explicitly
decided before enabling production card payments.

Launch configuration on September 19, 2026: Stripe subscription checkout and
collection are disabled. Testnet sellers receive an explicitly provisioned
launch entitlement; the dashboard must not present Stripe checkout or a billing
portal as an available onboarding action.

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
