output "http_api_url" {
  description = "HTTP API origin when the application deployment is enabled."
  value       = var.deployment_enabled ? aws_apigatewayv2_api.http[0].api_endpoint : null
}

output "mcp_url" {
  description = "Remote MCP endpoint when the application deployment is enabled."
  value       = var.deployment_enabled ? "${aws_apigatewayv2_api.http[0].api_endpoint}/mcp" : null
}
