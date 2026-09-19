# Seller integration recipe contract

Status: **Locked through STK-003**.

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
and atomic when more than one process can receive requests. The sandbox handler
returns the closed `agentpay.sandbox.v2` response containing the supplied
`routeId`, `routeVersion`, and `ready: true`.

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

## Maintained Go recipes

| Stack | Verification adapter | Raw-body integration |
|---|---|---|
| Go `net/http` | `Verifier.Middleware` | Wrap sandbox and fulfillment handlers before mux registration |
| Gin | `Verifier.Verify` | Read and restore `c.Request.Body` before binding |
| Echo | `Verifier.Verify` | Read and restore `c.Request().Body` before binding |
| Fiber | `Verifier.Verify` | Copy `c.Body()` and verify before `c.Next()` |

Go recipes use the pinned
`github.com/fourgeez/agentpay/verification/go@v0.1.0` package and run
`go test ./...`.

## Maintained Python recipes

| Stack | Verification adapter | Raw-body integration |
|---|---|---|
| FastAPI | `AgentPayASGIMiddleware` | Install outside FastAPI to capture and replay ASGI receive bytes |
| Starlette | `AgentPayASGIMiddleware` | Install outside Starlette to capture and replay ASGI receive bytes |
| Flask | `verify_request_sync` | Call `request.get_data(cache=True)` before JSON parsing |
| Django | `verify_request_sync` | Verify `request.body` in middleware before the paid view |

Python recipes use the pinned `agentpay-verify==0.1.0` package and run
`python -m unittest discover -v`. The package exposes separate asynchronous and
synchronous replay-store boundaries so ASGI and WSGI applications do not hide
event-loop behavior inside verification.

## Maintained extended-framework recipes

| Stack | Verification package | Verification adapter | Raw-body integration |
|---|---|---|---|
| ASP.NET Core | `AgentPay.Verify` `0.1.0` | `AgentPayVerificationMiddleware` | Enable buffering, verify captured bytes, then rewind `Request.Body` |
| Spring Boot | `com.agentpay:agentpay-verify-spring:0.1.0` | `AgentPayVerificationFilter` | Buffer the servlet input before Jackson or controller binding |
| Rails | `agentpay-verify` `0.1.0` | `AgentPay::VerificationMiddleware` | Read and replace `rack.input` before Rails parameter parsing |
| Laravel | `agentpay/verify` `0.1.0` | `AgentPayVerificationMiddleware` | Verify `getContent` bytes before request validation or controllers |

The package fixtures live under `verification/dotnet`, `verification/java`,
`verification/ruby`, and `verification/php`. The repository test runner uses
the package-native runtime or a pinned official runtime image and verifies
valid, modified, stale, replayed, and weak-secret cases before these stacks are
advertised as maintained.
