output "user_pool_id" {
  description = "Cognito seller user-pool ID."
  value       = aws_cognito_user_pool.seller.id
}

output "user_pool_client_id" {
  description = "Public Cognito app-client ID used only by the trusted web BFF."
  value       = aws_cognito_user_pool_client.seller_web.id
}

output "issuer" {
  description = "Exact Cognito issuer pinned by the Go token verifier."
  value       = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.seller.id}"
}
