# Seller request verification

Status: **Locked for AUT-006**.

AgentPay maintains equivalent verification packages for Go `net/http`, Node.js
middleware, and Python ASGI applications. Version 1 verifies the exact signing
contract in `SECURITY.md` using HMAC-SHA256 and constant-time comparison.

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
