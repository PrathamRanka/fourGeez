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
