# AgentPay seller verification packages

These packages implement the versioned seller-request signature contract in
`docs/SELLER_VERIFICATION.md`:

- `go`: standard-library `net/http` middleware plus maintained Gin, Echo, and
  Fiber recipes;
- `node`: TypeScript verification, an Express-compatible middleware requiring
  `request.rawBody`, and maintained setup recipes for Next.js, React/Vite,
  Remix, Nuxt, SvelteKit, Astro, Express, Fastify, and NestJS; and
- `python`: dependency-free verification, ASGI middleware for FastAPI and
  Starlette, and synchronous verification for Flask and Django recipes.

The included memory replay stores are for tests and single-process local
development. Production deployments must provide a shared atomic replay store.
