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
