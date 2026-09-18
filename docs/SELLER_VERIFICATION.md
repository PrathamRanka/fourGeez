# Seller request verification

Status: **Locked through STK-003**.

AgentPay maintains equivalent verification packages for Go, Node.js, Python,
.NET, Java, Ruby, and PHP applications. These packages support the maintained
Go and Python frameworks, JavaScript and TypeScript stacks, ASP.NET Core,
Spring Boot, Rails, and Laravel. Version 1 verifies the exact signing contract
in `SECURITY.md` using HMAC-SHA256 and constant-time comparison.

Each verifier requires a secret of at least 32 bytes and checks:

1. `X-AgentPay-Signature` is valid base64 and matches the canonical request.
2. `X-AgentPay-Timestamp` is RFC 3339 UTC and no more than five minutes old or
   in the future.
3. `X-AgentPay-Transaction-Id` is present and has not already been accepted.
4. The method, literal URL path, and lowercase SHA-256 hash of the exact raw
   request body match the signed values.

Replay storage is an explicit application boundary. The included in-memory
implementation is for tests and local development only. Production sellers
must use a shared atomic store when more than one process can receive requests.

Verification failure returns `401`; replay returns `409`; verifier or replay
store failure returns `503`. Middleware never logs or returns the secret,
signature, authorization headers, body, or internal error details.

The .NET package exposes ASP.NET Core middleware that buffers and rewinds the
request body. The Java package exposes a verifier and Spring filter primitive,
the Ruby package exposes Rack middleware for Rails, and the PHP package exposes
the verifier used by Laravel middleware. Their committed fixture tests cover a
valid signature, modified-body rejection, stale-request rejection, replay
rejection, and minimum secret length. Run all four with
`npm run test:verification:extended`; missing .NET, Ruby, or PHP runtimes use
pinned official Docker images, while Java 17 or newer is required locally.
