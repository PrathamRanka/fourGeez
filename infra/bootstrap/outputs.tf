output "state_bucket_name" {
  description = "Bucket value to place in the environment backend configuration."
  value       = aws_s3_bucket.terraform_state.id
}

output "state_bucket_region" {
  description = "Region value to place in the environment backend configuration."
  value       = var.aws_region
}

output "state_key" {
  description = "State object key for this environment."
  value       = "agentpay/${var.environment}/terraform.tfstate"
}
