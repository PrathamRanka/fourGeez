locals {
  common_tags = merge(
    {
      Environment = var.environment
      ManagedBy   = "Terraform"
      Project     = "AgentPay"
      Purpose     = "TerraformState"
      Repository  = "fourGeez"
    },
    var.additional_tags,
  )
}
