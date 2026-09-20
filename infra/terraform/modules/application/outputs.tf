output "http_api_url" {
  description = "HTTP API origin when the application deployment is enabled."
  value       = var.deployment_enabled ? aws_apigatewayv2_api.http[0].api_endpoint : null
}

output "mcp_url" {
  description = "Remote MCP endpoint when the application deployment is enabled."
  value       = var.deployment_enabled ? "${aws_apigatewayv2_api.http[0].api_endpoint}/mcp" : null
}

output "api_id" {
  description = "HTTP API identifier when the application deployment is enabled."
  value       = var.deployment_enabled ? aws_apigatewayv2_api.http[0].id : null
}

output "lambda_function_name" {
  description = "API Lambda function name when the application deployment is enabled."
  value       = var.deployment_enabled ? aws_lambda_function.api[0].function_name : null
}

output "operations_dashboard_name" {
  description = "CloudWatch operations dashboard name when the application deployment is enabled."
  value       = var.deployment_enabled ? aws_cloudwatch_dashboard.operations[0].dashboard_name : null
}

output "publication_outbox_dlq_url" {
  description = "DLQ URL for publication outbox records that exhaust bounded retries."
  value       = var.deployment_enabled ? aws_sqs_queue.publication_outbox_dlq[0].url : null
}
