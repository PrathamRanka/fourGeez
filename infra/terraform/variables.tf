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

variable "api_lambda_artifact_sha256" {
  description = "Base64-encoded SHA-256 of the reviewed Lambda ZIP. Required before deployment is enabled."
  type        = string
  default     = null
  nullable    = true

  validation {
    condition     = !var.api_deployment_enabled || var.api_lambda_artifact_sha256 != null
    error_message = "api_lambda_artifact_sha256 is required when api_deployment_enabled is true."
  }
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
