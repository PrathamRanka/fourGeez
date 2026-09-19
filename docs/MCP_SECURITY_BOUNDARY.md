# Cloud-authoritative MCP security boundary

Status: **Locked and implemented locally by LCH-010; deployed verification is pending**.

The Go runtime accepts project keys only at
`POST /v1/integration-access-tokens`, requires short-lived ES256 bearer
capabilities at `/mcp`, and requires cloud-issued one-time confirmation grants
for commercial mutations. Caller-supplied `confirmation.approved`,
`confirmation.summary`, and `confirmation.confirmedAt` fields are not accepted
as production authority. AWS-005, AWS-006, package distribution, and the M9
deployed release checks remain incomplete, so this boundary must not yet be
represented as a publicly available production integration.

## Boundary rule

Seller-hosted repositories, generated files, prompts, coding-agent hosts,
connectors, verification middleware, discovery copies, and fulfillment code
are untrusted. A seller may inspect, modify, remove, or fork all of them.
AgentPay therefore secures participation by keeping every network-authoritative
decision and signing key in the AgentPay cloud rather than by trying to disable
seller-owned code.

The official AgentPay MCP server is the cloud endpoint at `/mcp`. A local
connector is only a bounded credential exchanger and JSON-RPC relay. It is not
an authoritative MCP server, entitlement service, payment verifier, publisher,
or transaction executor.

## Authority split

| Capability | AgentPay cloud | Seller-controlled environment |
|---|---|---|
| Seller identity, subscription, entitlement epoch, quota | Authoritative | No authority; may display stale status |
| Project key validation and revocation | Authoritative | Stores the raw key in a server-side secret store |
| MCP access-token issuance and verification | Cloud-only signer and current-state checks | Holds a short-lived bearer token in process memory |
| Product configuration and publication | Validates and commits confirmed mutations | Proposes arguments and displays the result |
| Discovery status and signature | Publishes active documents or inactive tombstones | May host an untrusted cached copy |
| Purchase intent, policy, x402 challenge, verification, settlement | Authoritative | No authority |
| Transaction claim and execution capability | Conditional cloud write and cloud-only signer | Verifies the exact signed request and fulfills it |
| Receipt, evidence, audit, webhook, analytics | Authoritative records | Receives bounded signed outputs |
| Seller business logic and response | No ownership | Authoritative only for the upstream service output |

Seller code receives only public verification material where verification is
required. Private capability-signing keys, facilitator credentials, payment
verification authority, entitlement mutation authority, publication authority,
and official transaction state never leave the AgentPay control plane.

## Authentication and authorization

The project key is a bootstrap credential. It is accepted only by
`POST /v1/integration-access-tokens`, never by `/mcp`, seller control APIs,
buyer commerce APIs, paid routes, or seller fulfillment endpoints. The cloud
exchange verifies the keyed credential digest, current credential state,
current active entitlement, `now < accessEndsAt`, requested scope, quota, and
rate limits before issuing a 120-300 second access token.

Project-key installations use the AgentPay connector. Direct remote MCP OAuth
clients and OAuth protected-resource metadata are deferred in Lean V1. The
connector exchanges the project key for a bearer capability whose seller,
credential, scopes, audience, expiry, JTI, and entitlement epoch are
revalidated by the cloud on every request. Signature validity alone is
insufficient.

Every MCP operation derives seller identity from the verified capability. A
caller-supplied seller identifier, repository setting, manifest, prompt, or
tool argument cannot select another tenant or broaden a scope. Transaction-
critical authorization fails closed when current credential, entitlement,
revocation, quota, or replay state cannot be established.

## Cloud-issued mutation confirmation

MCP access scope permits a tool to request an operation; it does not prove that
the seller approved the exact commercial mutation. The authenticated seller
dashboard must show the canonical operation summary and exact arguments before
its trusted browser/BFF boundary calls
`POST /v1/sellers/{sellerId}/mcp-confirmation-grants`.

That endpoint issues an opaque 256-bit one-time `MCPConfirmationGrant` with a
maximum lifetime of five minutes. It is bound to:

- the authenticated `sellerId` and selected `credentialId`;
- one exact MCP tool name;
- one target type and target identifier;
- `argumentsSha256`, calculated from RFC 8785 canonical JSON with the
  `agentpay.mcp-mutation.v1` domain separator after excluding the grant and
  idempotency key;
- the expected seller or route resource version;
- a unique grant ID, issue time, and exclusive expiry; and
- the authenticated seller-session actor recorded in audit history.

The raw grant is returned only in the creation response with
`Cache-Control: no-store`. AgentPay persists only its keyed digest and binding
metadata. Reissuing the same binding atomically revokes any earlier unconsumed
grant, so a safe browser retry leaves at most one usable grant. The browser
hands the grant to the selected MCP interaction; the connector only relays it.
Caller-supplied approval booleans, summaries, and timestamps may remain
temporarily as display metadata during migration but are never authorization
inputs in production.

The cloud MCP mutation path validates the access token and then atomically
consumes the matching confirmation grant with the mutation idempotency
decision. Missing, expired, revoked, already-consumed, wrong-seller,
wrong-credential, wrong-tool, wrong-target, wrong-version, or wrong-arguments
grants fail closed before any domain mutation. An exact retry using the same
idempotency key returns the stored redacted result; it does not execute again.
Reuse with different canonical input returns `idempotency_conflict` or
`token_replayed` and records a denied audit event.

Confirmation-grant creation requires an authenticated seller browser session,
seller ownership, exact allowed Origin, double-submit CSRF validation, recent
reauthentication when policy requires it, and current active entitlement. MCP
bearer tokens, project keys, buyer credentials, model output, repository text,
and seller-hosted code cannot mint or approve a grant. Production deployment
and local repository edits remain outside AgentPay's authority and require the
seller's local tooling confirmation separately.

## Discovery and transaction authorization

Discovery answers only which products may be candidates. AgentPay-hosted
documents are short-lived and signed; seller-hosted manifests, `llms.txt`,
storefront copies, and MCP resources are untrusted hints. Neither a valid
discovery signature nor an active-looking cached document authorizes an
intent, x402 challenge, payment, settlement, or fulfillment.

Each commerce checkpoint independently loads current cloud state: seller
entitlement and `accessEndsAt`, route ownership and publication, verified
payment destination, immutable intent and frozen quote, approval state,
payment uniqueness and finality, and the exactly-once forwarding claim. An
execution capability is minted only after finalized payment and the cloud
claim. It is never derived from discovery or MCP confirmation.

## Cancellation and forks

At `accessEndsAt`, or immediately for suspension, closure, revocation, or fraud
quarantine, AgentPay denies new token exchange and MCP access, removes active
discovery, rejects new intents and challenges, and refuses new payment
verification or settlement. A seller may keep a local connector or fork
running, but that code receives no fresh cloud capability and cannot create an
official AgentPay transaction.

A fork may accept independent traffic or imitate AgentPay output on the
seller's own infrastructure. It cannot produce AgentPay discovery signatures,
MCP access tokens, x402 verification decisions, transaction identifiers,
execution capabilities, receipts, evidence, audit records, or dashboard state.
Traffic that bypasses the AgentPay cloud is a separate seller-operated system
and must not be represented as an AgentPay transaction.

The security model does not depend on obfuscation, license checks, connector
integrity, remote deletion, or a kill switch in seller-controlled code.

## Deployment and release gate

OpenAPI 0.6 and this document describe the implemented local runtime boundary.
The confirmation-grant store, authenticated issuance endpoint, atomic
consumption, short-lived MCP authorization, entitlement checks, and
fork-resistance tests are present in the repository. Public production use
still requires the AWS runtime deployment, Cognito configuration, registry or
other approved connector/package distribution, and the M9 deployed revocation,
cancellation, fork, sandbox, and end-to-end checks. A deployment with any of
those authorization dependencies unavailable must fail closed rather than
falling back to project-key access or caller-asserted confirmation.

AsyncAPI is unchanged by this decision: confirmation grants do not create a
public subscription channel. The seller dashboard may refresh pending state
over authenticated HTTP, while mutation success or failure is returned through
the originating MCP call and durable audit history.
