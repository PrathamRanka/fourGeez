# AgentPay seller verification packages

These packages implement the versioned seller-request signature contract in
`docs/SELLER_VERIFICATION.md`:

- `go`: standard-library `net/http` middleware;
- `node`: TypeScript verification and Express-compatible middleware requiring
  `request.rawBody`; and
- `python`: dependency-free verification and ASGI middleware.

The included memory replay stores are for tests and single-process local
development. Production deployments must provide a shared atomic replay store.
