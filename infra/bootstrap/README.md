# Terraform state bootstrap

This root creates the protected S3 bucket used by the main Terraform root. It
intentionally uses local state because a remote backend cannot create itself.
The bootstrap state file is sensitive operational data: store it securely and
never commit it.

S3 native lock files are used through `use_lockfile = true`; AgentPay does not
create the legacy DynamoDB locking table.

Follow `docs/AWS_SETUP.md` for caller verification, apply, backend migration,
and post-migration checks.
