output "deployment" {
  description = "Foundation identity consumed as resources are added in AWS-001 through AWS-004."
  value = {
    aws_account_id  = var.aws_account_id
    aws_region      = var.aws_region
    environment     = var.environment
    resource_prefix = local.resource_prefix
    tags            = var.tags
  }
}

output "table_name" {
  description = "DynamoDB table used by the AgentPay runtime."
  value       = aws_dynamodb_table.agentpay.name
}

output "table_arn" {
  description = "DynamoDB table ARN used by least-privilege runtime policies."
  value       = aws_dynamodb_table.agentpay.arn
}

output "evidence_bucket_name" {
  description = "Object Lock evidence bucket name."
  value       = aws_s3_bucket.evidence.id
}

output "evidence_kms_key_id" {
  description = "Asymmetric KMS key ID used to sign evidence hashes."
  value       = aws_kms_key.evidence_signing.key_id
}

output "evidence_kms_key_arn" {
  description = "Asymmetric KMS key ARN used by evidence IAM policies."
  value       = aws_kms_key.evidence_signing.arn
}

output "capability_signing_key_id" {
  description = "Active asymmetric KMS key ID used for access, discovery, and execution capabilities."
  value       = aws_kms_key.capability_signing[var.active_capability_signing_key_version].key_id
}

output "capability_verification_key_ids" {
  description = "Versioned capability keys published through JWKS during rotation overlap."
  value       = { for version, key in aws_kms_key.capability_signing : version => key.key_id }
}

output "credential_pepper_secret_arn" {
  description = "Secrets Manager container ARN for the project-credential digest pepper."
  value       = aws_secretsmanager_secret.credential_pepper.arn
}

output "confirmation_grant_pepper_secret_arn" {
  description = "Secrets Manager container ARN for the confirmation-grant digest pepper."
  value       = aws_secretsmanager_secret.confirmation_grant_pepper.arn
}

output "application_secrets_kms_key_id" {
  description = "Symmetric KMS key ID for secret and replay-envelope encryption."
  value       = aws_kms_key.application_secrets.key_id
}

output "api_runtime_role_arn" {
  description = "Least-privilege IAM role ARN for the API Lambda."
  value       = aws_iam_role.api_runtime.arn
}

output "api_runtime_role_name" {
  description = "IAM role name used for AWS-005 runtime log permissions."
  value       = aws_iam_role.api_runtime.name
}

output "evidence_verifier_role_arn" {
  description = "Read-only IAM role ARN for evidence verification."
  value       = aws_iam_role.evidence_verifier.arn
}
