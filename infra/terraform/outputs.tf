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

output "table_stream_arn" {
  description = "DynamoDB stream ARN for publication outbox processing."
  value       = module.foundation.table_stream_arn
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

output "seller_user_pool_id" {
  description = "Cognito user pool for AGENTPAY_SELLER_USER_POOL_ID."
  value       = module.identity.user_pool_id
}

output "seller_user_pool_client_id" {
  description = "Cognito app client for AGENTPAY_SELLER_USER_POOL_CLIENT_ID."
  value       = module.identity.user_pool_client_id
}

output "seller_identity_issuer" {
  description = "Exact Cognito issuer pinned by the Go and API Gateway verifiers."
  value       = module.identity.issuer
}

output "http_api_url" {
  description = "HTTP API origin for AGENTPAY_HTTP_API_URL; null while AWS-005 deployment is disabled."
  value       = module.application.http_api_url
}

output "mcp_url" {
  description = "Remote MCP endpoint for AGENTPAY_MCP_URL; null while AWS-005 deployment is disabled."
  value       = module.application.mcp_url
}

output "environment" {
  description = "Environment name used by deployment automation."
  value       = var.environment
}

output "project_name" {
  description = "Stable project name used to bind operator tooling to this Terraform environment."
  value       = var.project_name
}

output "aws_region" {
  description = "AWS region used by deployment automation."
  value       = var.aws_region
}

output "web_origin" {
  description = "Canonical web origin configured in API CORS."
  value       = var.web_origin
}

output "api_id" {
  description = "HTTP API identifier; null while AWS-005 is disabled."
  value       = module.application.api_id
}

output "api_lambda_function_name" {
  description = "API Lambda function name; null while AWS-005 is disabled."
  value       = module.application.lambda_function_name
}

output "operations_dashboard_name" {
  description = "CloudWatch seller-operations dashboard; null while AWS-005 is disabled."
  value       = module.application.operations_dashboard_name
}

output "publication_outbox_dlq_url" {
  description = "Publication outbox dead-letter queue URL."
  value       = module.application.publication_outbox_dlq_url
}

output "launch_entitlement_operator_role_arn" {
  description = "Optional least-privilege role for audited launch-entitlement operations."
  value       = length(aws_iam_role.launch_entitlement_operator) == 1 ? aws_iam_role.launch_entitlement_operator[0].arn : null
}

output "vercel_environment" {
  description = "Exact non-secret server-side values for the Vercel web deployment."
  value = {
    AGENTPAY_ENV                        = var.environment
    AGENTPAY_IDENTITY_MODE              = "cognito"
    AGENTPAY_WEB_ORIGIN                 = var.web_origin
    AGENTPAY_API_ORIGIN                 = module.application.http_api_url
    AWS_REGION                          = var.aws_region
    AGENTPAY_SELLER_USER_POOL_CLIENT_ID = module.identity.user_pool_client_id
  }
}
