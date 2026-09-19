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
