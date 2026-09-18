# Seller integration recipe contract

Status: **Locked through STK-001**.

Each maintained stack has a versioned `agentpay.recipe.v1` entry included in
setup bundle v2. A recipe identifies the pinned verification package and
adapter, the framework-specific raw-body strategy, required middleware order,
sandbox route, storefront files, discovery files, and focused test command.

The required middleware order is:

1. `capture_raw_body`;
2. `verify_agentpay_signature`;
3. `claim_replay_identifier`; and
4. `fulfillment_handler`.

Every recipe generates `robots.txt`, `sitemap.xml`, `llms.txt`, and
`manifest.json`, and places `POST /.well-known/agentpay/sandbox` behind the same
verification boundary as fulfillment. Production replay storage must be shared
and atomic when more than one process can receive requests.

## Maintained JavaScript and TypeScript recipes

| Stack | Verification adapter | Raw-body integration | Primary generated locations |
|---|---|---|---|
| Next.js | `verifyAgentPayRequest` | Read `request.arrayBuffer()` before decoding | `app/layout.tsx`, product page, Route Handler sandbox |
| React/Vite | `verifyAgentPayRequest` | Verify in the companion Node API before JSON parsing | `index.html`, product route, server integration |
| Remix | `verifyAgentPayRequest` | Read `request.arrayBuffer()` before request helpers | root metadata, product route, resource-route sandbox |
| Nuxt | `verifyAgentPayRequest` | Capture Nitro raw body before parsing | `nuxt.config.ts`, product page, server sandbox route |
| SvelteKit | `verifyAgentPayRequest` | Read `Request.arrayBuffer()` before request helpers | root layout, product page, `+server` sandbox route |
| Astro | `verifyAgentPayRequest` | Read `Astro.request.arrayBuffer()` before decoding | layout, product page, endpoint sandbox |
| Express | `createAgentPayMiddleware` | Populate `request.rawBody` before JSON middleware | storefront router and sandbox router |
| Fastify | `verifyAgentPayRequest` | Capture payload in `onRequest` or `preParsing` | storefront plugin and sandbox route |
| NestJS | `verifyAgentPayRequest` | Enable raw body and verify in a guard before controllers | guard, storefront controller, sandbox controller |

Focused fixtures under `internal/integrations/recipes/testdata` prove exact
stack detection, maintained-tier advertising, package selection, adapter
selection, metadata file placement, middleware order, sandbox coverage, and
discovery outputs. A detected stack is not considered maintained unless that
fixture passes.
