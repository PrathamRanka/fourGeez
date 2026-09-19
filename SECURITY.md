# Security policy

## Supported versions

AgentPay is a pre-production development and x402 testnet project. Only the
current `main` branch receives security fixes. No released version is currently
supported for production or real-funds use.

## Reporting a vulnerability

Do not open a public issue, discussion, or pull request containing exploit
details, credentials, personal data, or sensitive reproduction material.

Use GitHub private vulnerability reporting from the repository's **Security**
tab when a **Report a vulnerability** action is available. The repository
owners must enable that feature before public launch.

If that private action is not available, this repository does not currently
publish a verified private security contact. Report only that a private channel
is needed, without vulnerability details, through an existing trusted channel
you already have with an owner. Publishing a dedicated security contact is a
release blocker; this file does not invent an email address or response SLA.

Include, when safe:

- affected commit or deployed environment;
- impact and prerequisites;
- minimal reproduction steps;
- whether credentials, money, personal data, or cross-tenant access are at
  risk; and
- suggested mitigations, if known.

The owners will coordinate validation and remediation privately. No fixed
response or disclosure timeline is promised until an operating entity,
staffed contact, and incident-response policy are approved.

## Scope and expectations

Review the engineering threat model in `docs/SECURITY.md`. Never test with real
funds, attack third-party systems, access another person's data, degrade the
service, or retain sensitive data. This policy is not a legal safe-harbor
promise; qualified counsel must approve any future disclosure or bounty terms.
