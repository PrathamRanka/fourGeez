# AgentPay seller verification packages

These packages implement the versioned seller-request signature contract in
`docs/SELLER_VERIFICATION.md`:

- `go`: standard-library `net/http` middleware plus maintained Gin, Echo, and
  Fiber recipes;
- `node`: TypeScript verification, an Express-compatible middleware requiring
  `request.rawBody`, and maintained setup recipes for Next.js, React/Vite,
  Remix, Nuxt, SvelteKit, Astro, Express, Fastify, and NestJS; and
- `python`: dependency-free verification, ASGI middleware for FastAPI and
  Starlette, and synchronous verification for Flask and Django recipes;
- `dotnet`: ASP.NET Core request buffering and verification middleware;
- `java`: a dependency-free verifier and Spring Boot filter primitive;
- `ruby`: Rack verification middleware for Rails; and
- `php`: a verifier and middleware primitive for Laravel.

The included memory replay stores are for tests and single-process local
development. Production deployments must provide a shared atomic replay store.

Run the extended package fixtures with:

```powershell
npm run test:verification:extended
```

The runner uses installed runtimes when available. It otherwise uses pinned
official .NET, Ruby, and PHP Docker images. Java 17 or newer must be installed.
