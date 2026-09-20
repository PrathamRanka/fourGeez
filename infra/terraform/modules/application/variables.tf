variable "deployment_enabled" {
  description = "Whether to create the AWS-005 application runtime."
  type        = bool
}

variable "environment" {
  description = "AgentPay environment name."
  type        = string
}

variable "project_name" {
  description = "Stable project name used for resource prefixes."
  type        = string
}

variable "web_origin" {
  description = "Canonical frontend origin allowed by HTTP API CORS."
  type        = string
}

variable "public_api_origin" {
  description = "Stable branded public API origin emitted by the Lambda runtime."
  type        = string
}

variable "payment_mode" {
  description = "Payment adapter mode passed to the API runtime."
  type        = string
}

variable "lambda_artifact_path" {
  description = "Path to the reviewed Lambda ZIP."
  type        = string
}

variable "lambda_artifact_sha256" {
  description = "Base64-encoded SHA-256 of the reviewed Lambda ZIP."
  type        = string
  nullable    = true
}

variable "lambda_memory_size_mb" {
  description = "Lambda memory allocation."
  type        = number
}

variable "lambda_timeout_seconds" {
  description = "Lambda invocation timeout."
  type        = number
}

variable "lambda_reserved_concurrency" {
  description = "Lambda reserved concurrency cost guard."
  type        = number
}

variable "throttling_burst_limit" {
  description = "HTTP API default-route burst limit."
  type        = number
}

variable "throttling_rate_limit" {
  description = "HTTP API default-route steady-state rate limit."
  type        = number
}

variable "log_retention_days" {
  description = "CloudWatch log retention."
  type        = number
}

variable "alarm_action_arns" {
  description = "SNS topic ARNs notified when an operational alarm changes state."
  type        = list(string)
  default     = []
}

variable "api_p95_latency_alarm_ms" {
  description = "Pilot HTTP API p95 latency threshold in milliseconds."
  type        = number
  default     = 2000
}

variable "lambda_p95_duration_alarm_ms" {
  description = "Pilot Lambda p95 duration threshold in milliseconds."
  type        = number
  default     = 12000
}

variable "api_runtime_role_arn" {
  description = "Least-privilege role attached to the API Lambda."
  type        = string
}

variable "api_runtime_role_name" {
  description = "API Lambda role name used for scoped log permissions."
  type        = string
}

variable "table_name" {
  description = "AgentPay DynamoDB table name."
  type        = string
}

variable "table_stream_arn" {
  description = "DynamoDB stream ARN for publication outbox processing."
  type        = string
}

variable "evidence_bucket_name" {
  description = "Protected evidence bucket name."
  type        = string
}

variable "evidence_kms_key_id" {
  description = "Evidence signing KMS key ID."
  type        = string
}

variable "capability_signing_key_id" {
  description = "Active capability signing KMS key ID."
  type        = string
}

variable "capability_verification_key_ids" {
  description = "Versioned capability verification KMS key IDs."
  type        = map(string)
}

variable "application_secrets_kms_key_id" {
  description = "Application envelope-encryption KMS key ID."
  type        = string
}

variable "credential_pepper_secret_arn" {
  description = "Project-credential pepper container ARN."
  type        = string
}

variable "confirmation_grant_pepper_secret_arn" {
  description = "Confirmation-grant pepper container ARN."
  type        = string
}

variable "seller_identity_issuer" {
  description = "Exact Cognito issuer for seller access tokens."
  type        = string
}

variable "seller_user_pool_id" {
  description = "Cognito seller user-pool ID."
  type        = string
}

variable "seller_user_pool_client_id" {
  description = "Cognito seller web app-client ID."
  type        = string
}

variable "facilitator_url" {
  description = "x402 testnet facilitator origin."
  type        = string
}

variable "x402_network" {
  description = "Allowed x402 testnet network identifier."
  type        = string
}

variable "x402_asset" {
  description = "Allowed x402 testnet asset identifier."
  type        = string
}
