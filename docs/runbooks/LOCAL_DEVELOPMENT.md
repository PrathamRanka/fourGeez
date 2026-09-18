# Local development runtime

Run the complete disposable local process graph from the repository root:

```powershell
npm run dev:local
```

The launcher starts and supervises:

- Next.js at `http://localhost:3000`;
- the real Go API at `http://127.0.0.1:8080`;
- the demo seller at `http://127.0.0.1:8090`; and
- the local mock facilitator at `http://127.0.0.1:8091`.

The Go API uses in-memory repositories, mock payment behavior, ephemeral local
credentials, and the explicitly labeled deterministic buyer fallback. The
launcher prints the verified URLs only after every process passes its readiness
probe.

Process liveness is available at `GET /health/live`. Transaction-path
readiness is available at `GET /health/ready` and fails with `503` unless the
configured in-memory repository, evidence and seller-request signing, mock
x402 facilitator, and demo seller forwarding target all pass bounded checks.
Responses report only dependency names and
`ready`/`unavailable`; dependency errors, URLs, response bodies, and credentials
are never returned. `GET /health` remains the legacy liveness contract.

The `launch-ready` profile is loaded automatically. It includes a complete demo
seller, an incomplete seller, fixed-price products,
representative transaction and evidence states, webhook delivery history, and
asset/network-separated analytics. Inspect its stable public identifiers at:

```text
GET http://127.0.0.1:8080/__dev/seed-profile
```

Open `http://localhost:3000/sign-in` and use the local-only launch-ready seller:

```text
Email: pratham@agentpay.local
Password: AgentPayLocalDemo2026
```

The account is already mapped to the seeded `demo-seller` storefront, so the
dashboard opens with products, transactions, evidence, disputes, integration
state, and asset/network-separated analytics. It exists only for the lifetime
of the local supervisor process.

Clear all seed-owned in-memory state and restore the profile with:

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8080/__dev/seed-profile/reset
```

The profile and reset route exist only in the `agentpay_dev` Go build selected
by the launcher. The same environment setting causes a production build to
fail startup rather than seed data.

Requirements are Node.js 24 or newer, npm 11 or newer, and Go 1.26 or newer.
Ports 3000, 8080, 8090, and 8091 must be free. The web and API ports may be
overridden with `AGENTPAY_LOCAL_WEB_PORT` and `AGENTPAY_LOCAL_API_PORT`; the demo
seller and mock facilitator retain their fixed local fixture ports.

Press Ctrl+C once to stop the runtime. On Windows, a detached cleanup guardian
tracks wrapper descendants so that `go run`, npm, Next.js, and their generated
child processes are all terminated even when the command shell exits first.
All application state and ephemeral credentials disappear with the process
tree; no cleanup command is required.
