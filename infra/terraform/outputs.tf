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
