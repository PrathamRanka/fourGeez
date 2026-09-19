variable "environment" {
  description = "AgentPay environment name."
  type        = string
}

variable "project_name" {
  description = "Stable project name used for Cognito resource names."
  type        = string
}

variable "aws_region" {
  description = "AWS region containing the seller user pool."
  type        = string
}

variable "self_registration_enabled" {
  description = "Whether sellers may create their own accounts in this environment."
  type        = bool
}

variable "tags" {
  description = "Common tags for supported Cognito resources."
  type        = map(string)
  default     = {}
}
