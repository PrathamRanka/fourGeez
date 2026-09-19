output "deployment" {
  description = "Validated deployment identity used by later AWS milestones."
  value       = module.foundation.deployment
}

output "table_name" {
  description = "DynamoDB table name for AGENTPAY_TABLE_NAME."
  value       = module.foundation.table_name
}

output "table_arn" {
  description = "DynamoDB table ARN for runtime IAM composition."
  value       = module.foundation.table_arn
}

output "evidence_bucket_name" {
  description = "S3 bucket name for AGENTPAY_EVIDENCE_BUCKET."
  value       = module.foundation.evidence_bucket_name
}

output "evidence_kms_key_id" {
  description = "KMS key ID for AGENTPAY_EVIDENCE_KMS_KEY_ID."
  value       = module.foundation.evidence_kms_key_id
}

output "evidence_kms_key_arn" {
  description = "KMS evidence-signing key ARN for runtime IAM composition."
  value       = module.foundation.evidence_kms_key_arn
}

output "capability_signing_key_id" {
  description = "Active KMS key ID used by the production capability signer."
  value       = module.foundation.capability_signing_key_id
}

output "capability_verification_key_ids" {
  description = "Version-to-KMS-key map published by the JWKS provider."
  value       = module.foundation.capability_verification_key_ids
}

output "credential_pepper_secret_arn" {
  description = "Secret ARN loaded by the production project-key digester."
  value       = module.foundation.credential_pepper_secret_arn
}

output "confirmation_grant_pepper_secret_arn" {
  description = "Secret ARN loaded by the production confirmation-grant digester."
  value       = module.foundation.confirmation_grant_pepper_secret_arn
}

output "application_secrets_kms_key_id" {
  description = "KMS key ID used for secret and replay-envelope encryption."
  value       = module.foundation.application_secrets_kms_key_id
}

output "api_runtime_role_arn" {
  description = "IAM role ARN attached to the API Lambda in AWS-005."
  value       = module.foundation.api_runtime_role_arn
}

output "evidence_verifier_role_arn" {
  description = "IAM role ARN reserved for read-only evidence verification."
  value       = module.foundation.evidence_verifier_role_arn
}

output "http_api_url" {
  description = "HTTP API origin for AGENTPAY_HTTP_API_URL; null while AWS-005 deployment is disabled."
  value       = module.application.http_api_url
}

output "mcp_url" {
  description = "Remote MCP endpoint for AGENTPAY_MCP_URL; null while AWS-005 deployment is disabled."
  value       = module.application.mcp_url
}
