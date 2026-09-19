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
