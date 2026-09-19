locals {
  common_tags = merge(
    {
      Environment = var.environment
      ManagedBy   = "Terraform"
      Project     = "AgentPay"
      Repository  = "fourGeez"
    },
    var.additional_tags,
  )
}
