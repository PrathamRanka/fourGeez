variable "aws_account_id" {
  description = "Twelve-digit AWS account that Terraform is allowed to modify."
  type        = string

  validation {
    condition     = can(regex("^[0-9]{12}$", var.aws_account_id))
    error_message = "aws_account_id must contain exactly twelve digits."
  }
}

variable "aws_region" {
  description = "AWS region for the AgentPay environment."
  type        = string
  default     = "ap-south-1"

  validation {
    condition     = can(regex("^[a-z]{2}(-gov)?-[a-z]+-[0-9]+$", var.aws_region))
    error_message = "aws_region must be a valid AWS region identifier."
  }
}

variable "environment" {
  description = "AgentPay deployment environment."
  type        = string

  validation {
    condition     = contains(["dev", "demo", "prod"], var.environment)
    error_message = "environment must be dev, demo, or prod."
  }
}

variable "project_name" {
  description = "Stable project name used in resource names and tags."
  type        = string
  default     = "agentpay"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,31}$", var.project_name))
    error_message = "project_name must be a lowercase AWS-safe name between 3 and 32 characters."
  }
}

variable "additional_tags" {
  description = "Non-sensitive tags added to every supported AWS resource."
  type        = map(string)
  default     = {}
}

variable "evidence_retention_days" {
  description = "Default Object Lock retention for evidence objects."
  type        = number
  default     = 30

  validation {
    condition     = var.evidence_retention_days >= 1 && var.evidence_retention_days <= 3650
    error_message = "evidence_retention_days must be between 1 and 3650."
  }
}

variable "capability_signing_key_versions" {
  description = "Version labels retained in JWKS during capability-key rotation."
  type        = set(string)
  default     = ["v1"]

  validation {
    condition = alltrue([
      for version in var.capability_signing_key_versions : can(regex("^v[1-9][0-9]*$", version))
    ]) && length(var.capability_signing_key_versions) > 0
    error_message = "capability_signing_key_versions must contain one or more v-prefixed positive integers."
  }
}

variable "active_capability_signing_key_version" {
  description = "Version label targeted by the current capability-signing alias."
  type        = string
  default     = "v1"

  validation {
    condition     = contains(var.capability_signing_key_versions, var.active_capability_signing_key_version)
    error_message = "active_capability_signing_key_version must exist in capability_signing_key_versions."
  }
}

variable "web_origin" {
  description = "Canonical browser origin allowed by API CORS and secure session checks."
  type        = string
  default     = "https://agentpay.prathamranka.in"

  validation {
    condition     = var.web_origin == "https://agentpay.prathamranka.in"
    error_message = "web_origin must be the canonical AgentPay production origin."
  }
}

variable "seller_self_registration_enabled" {
  description = "Whether Cognito permits seller self-registration in this environment."
  type        = bool
  default     = true
}

variable "budget_alert_email" {
  description = "Optional recipient for development account cost alerts."
  type        = string
  default     = null
  nullable    = true

  validation {
    condition = (
      var.budget_alert_email == null ||
      can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.budget_alert_email))
    )
    error_message = "budget_alert_email must be null or a valid email address."
  }
}

variable "monthly_budget_limit_usd" {
  description = "Monthly account cost budget in US dollars."
  type        = number
  default     = 10

  validation {
    condition     = var.monthly_budget_limit_usd >= 1 && var.monthly_budget_limit_usd <= 100
    error_message = "monthly_budget_limit_usd must be between 1 and 100."
  }
}

variable "payment_mode" {
  description = "Locked seller-first payment mode; mainnet and card rails remain disabled."
  type        = string
  default     = "x402"

  validation {
    condition     = var.payment_mode == "x402"
    error_message = "payment_mode must remain x402 for the testnet deployment."
  }
}

variable "api_deployment_enabled" {
  description = "Creates the AWS-005 Lambda and HTTP API only after durable runtime readiness passes."
  type        = bool
  default     = false
}

variable "api_lambda_artifact_path" {
  description = "Path to the reviewed ARM64 Lambda ZIP. Ignored while api_deployment_enabled is false."
  type        = string
  default     = "dist/agentpay-api.zip"
}

variable "api_memory_size_mb" {
  description = "Memory allocated to the API Lambda."
  type        = number
  default     = 512

  validation {
    condition     = var.api_memory_size_mb >= 128 && var.api_memory_size_mb <= 10240
    error_message = "api_memory_size_mb must be between 128 and 10240."
  }
}

variable "x402_facilitator_url" {
  description = "Locked x402 testnet facilitator origin."
  type        = string
  default     = "https://x402.org/facilitator"

  validation {
    condition     = var.x402_facilitator_url == "https://x402.org/facilitator"
    error_message = "x402_facilitator_url must remain the reviewed testnet facilitator for this release."
  }
}

variable "x402_network" {
  description = "Locked CAIP-2 network identifier for Base Sepolia."
  type        = string
  default     = "eip155:84532"

  validation {
    condition     = var.x402_network == "eip155:84532"
    error_message = "x402_network must remain Base Sepolia for this release."
  }
}

variable "x402_asset" {
  description = "Locked Base Sepolia USDC contract address."
  type        = string
  default     = "0x036CbD53842c5426634e7929541eC2318f3dCF7e"

  validation {
    condition     = lower(var.x402_asset) == lower("0x036CbD53842c5426634e7929541eC2318f3dCF7e")
    error_message = "x402_asset must remain the reviewed Base Sepolia USDC contract for this release."
  }
}

variable "api_timeout_seconds" {
  description = "Maximum API Lambda invocation duration."
  type        = number
  default     = 15

  validation {
    condition     = var.api_timeout_seconds >= 1 && var.api_timeout_seconds <= 29
    error_message = "api_timeout_seconds must be between 1 and 29 for the HTTP API integration."
  }
}

variable "api_reserved_concurrency" {
  description = "Cost and load guard for simultaneous API Lambda executions."
  type        = number
  default     = 5

  validation {
    condition     = var.api_reserved_concurrency >= 1 && var.api_reserved_concurrency <= 50
    error_message = "api_reserved_concurrency must be between 1 and 50."
  }
}

variable "api_throttling_burst_limit" {
  description = "HTTP API burst request limit for the default route."
  type        = number
  default     = 20

  validation {
    condition     = var.api_throttling_burst_limit >= 1 && var.api_throttling_burst_limit <= 100
    error_message = "api_throttling_burst_limit must be between 1 and 100."
  }
}

variable "api_throttling_rate_limit" {
  description = "HTTP API steady-state requests per second for the default route."
  type        = number
  default     = 10

  validation {
    condition     = var.api_throttling_rate_limit >= 1 && var.api_throttling_rate_limit <= 100
    error_message = "api_throttling_rate_limit must be between 1 and 100."
  }
}

variable "api_log_retention_days" {
  description = "CloudWatch retention for development API and Lambda logs."
  type        = number
  default     = 7

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90], var.api_log_retention_days)
    error_message = "api_log_retention_days must be a supported short CloudWatch retention period."
  }
}

variable "operational_alarm_action_arns" {
  description = "Optional SNS topic ARNs for seller-path operational alarms."
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for arn in var.operational_alarm_action_arns : can(regex("^arn:aws:sns:[a-z0-9-]+:[0-9]{12}:[A-Za-z0-9_-]+$", arn))
    ])
    error_message = "operational_alarm_action_arns must contain only SNS topic ARNs."
  }
}

variable "launch_entitlement_operator_principal_arns" {
  description = "IAM principals allowed to assume the narrow launch-entitlement operator role."
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for arn in var.launch_entitlement_operator_principal_arns : can(regex("^arn:aws:iam::[0-9]{12}:(user|role)/[A-Za-z0-9+=,.@_/-]+$", arn))
    ])
    error_message = "launch_entitlement_operator_principal_arns must contain IAM user or role ARNs; root is not accepted."
  }
}

variable "api_p95_latency_alarm_ms" {
  description = "HTTP API p95 latency alarm threshold in milliseconds."
  type        = number
  default     = 2000

  validation {
    condition     = var.api_p95_latency_alarm_ms >= 300 && var.api_p95_latency_alarm_ms <= 10000
    error_message = "api_p95_latency_alarm_ms must be between 300 and 10000."
  }
}

variable "lambda_p95_duration_alarm_ms" {
  description = "Lambda p95 duration alarm threshold in milliseconds."
  type        = number
  default     = 12000

  validation {
    condition     = var.lambda_p95_duration_alarm_ms >= 1000 && var.lambda_p95_duration_alarm_ms < var.api_timeout_seconds * 1000
    error_message = "lambda_p95_duration_alarm_ms must be at least 1000 and below the Lambda timeout."
  }
}
