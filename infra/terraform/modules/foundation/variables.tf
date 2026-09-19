variable "aws_account_id" {
  description = "AWS account selected by the root provider guard."
  type        = string
}

variable "aws_region" {
  description = "AWS region selected for this environment."
  type        = string
}

variable "environment" {
  description = "AgentPay environment name."
  type        = string
}

variable "project_name" {
  description = "Stable project name used for resource prefixes."
  type        = string
}

variable "tags" {
  description = "Common non-sensitive resource tags."
  type        = map(string)
}

variable "evidence_retention_days" {
  description = "Default Object Lock retention for evidence objects."
  type        = number
}

variable "capability_signing_key_versions" {
  description = "Version labels retained in JWKS during capability-key rotation."
  type        = set(string)
}

variable "active_capability_signing_key_version" {
  description = "Version label targeted by the current capability-signing alias."
  type        = string
}
