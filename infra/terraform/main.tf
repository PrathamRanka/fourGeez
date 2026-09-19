module "foundation" {
  source = "./modules/foundation"

  aws_account_id          = var.aws_account_id
  aws_region              = var.aws_region
  environment             = var.environment
  project_name            = var.project_name
  evidence_retention_days = var.evidence_retention_days
  tags                    = local.common_tags
}
