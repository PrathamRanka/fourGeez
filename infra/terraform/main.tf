module "foundation" {
  source = "./modules/foundation"

  aws_account_id                        = var.aws_account_id
  aws_region                            = var.aws_region
  environment                           = var.environment
  project_name                          = var.project_name
  evidence_retention_days               = var.evidence_retention_days
  capability_signing_key_versions       = var.capability_signing_key_versions
  active_capability_signing_key_version = var.active_capability_signing_key_version
  tags                                  = local.common_tags
}

module "application" {
  source = "./modules/application"

  deployment_enabled                   = var.api_deployment_enabled
  environment                          = var.environment
  project_name                         = var.project_name
  web_origin                           = var.web_origin
  lambda_artifact_path                 = var.api_lambda_artifact_path
  lambda_artifact_sha256               = var.api_lambda_artifact_sha256
  lambda_memory_size_mb                = var.api_memory_size_mb
  lambda_timeout_seconds               = var.api_timeout_seconds
  lambda_reserved_concurrency          = var.api_reserved_concurrency
  throttling_burst_limit               = var.api_throttling_burst_limit
  throttling_rate_limit                = var.api_throttling_rate_limit
  log_retention_days                   = var.api_log_retention_days
  api_runtime_role_arn                 = module.foundation.api_runtime_role_arn
  api_runtime_role_name                = module.foundation.api_runtime_role_name
  table_name                           = module.foundation.table_name
  evidence_bucket_name                 = module.foundation.evidence_bucket_name
  evidence_kms_key_id                  = module.foundation.evidence_kms_key_id
  capability_signing_key_id            = module.foundation.capability_signing_key_id
  capability_verification_key_ids      = module.foundation.capability_verification_key_ids
  application_secrets_kms_key_id       = module.foundation.application_secrets_kms_key_id
  credential_pepper_secret_arn         = module.foundation.credential_pepper_secret_arn
  confirmation_grant_pepper_secret_arn = module.foundation.confirmation_grant_pepper_secret_arn
}
