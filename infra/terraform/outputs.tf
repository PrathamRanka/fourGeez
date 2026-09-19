output "deployment" {
  description = "Validated deployment identity used by later AWS milestones."
  value       = module.foundation.deployment
}
