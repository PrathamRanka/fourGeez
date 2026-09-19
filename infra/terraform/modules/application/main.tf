locals {
  resource_prefix       = "${var.project_name}-${var.environment}"
  lambda_log_group_name = "/aws/lambda/${local.resource_prefix}-api"
}

resource "aws_cloudwatch_log_group" "lambda" {
  count = var.deployment_enabled ? 1 : 0

  name              = local.lambda_log_group_name
  retention_in_days = var.log_retention_days
}

resource "aws_cloudwatch_log_group" "api" {
  count = var.deployment_enabled ? 1 : 0

  name              = "/aws/apigateway/${local.resource_prefix}-http-api"
  retention_in_days = var.log_retention_days
}

data "aws_iam_policy_document" "runtime_logs" {
  count = var.deployment_enabled ? 1 : 0

  statement {
    sid       = "WriteLambdaLogs"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.lambda[0].arn}:*"]
  }
}

resource "aws_iam_role_policy" "runtime_logs" {
  count = var.deployment_enabled ? 1 : 0

  name   = "${local.resource_prefix}-api-logs"
  role   = var.api_runtime_role_name
  policy = data.aws_iam_policy_document.runtime_logs[0].json
}

resource "aws_apigatewayv2_api" "http" {
  count = var.deployment_enabled ? 1 : 0

  name          = "${local.resource_prefix}-http-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_credentials = true
    allow_headers = [
      "authorization",
      "content-type",
      "idempotency-key",
      "x-agentpay-csrf",
      "x-payment",
    ]
    allow_methods  = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
    allow_origins  = [var.web_origin]
    expose_headers = ["payment-required", "x-payment-response"]
    max_age        = 300
  }
}

resource "aws_lambda_function" "api" {
  count = var.deployment_enabled ? 1 : 0

  function_name = "${local.resource_prefix}-api"
  description   = "AgentPay REST and MCP runtime"
  role          = var.api_runtime_role_arn
  architectures = ["arm64"]
  runtime       = "provided.al2023"
  handler       = "bootstrap"
  filename      = var.lambda_artifact_path

  source_code_hash               = var.lambda_artifact_sha256
  memory_size                    = var.lambda_memory_size_mb
  timeout                        = var.lambda_timeout_seconds
  reserved_concurrent_executions = var.lambda_reserved_concurrency

  environment {
    variables = {
      AGENTPAY_ENV                                  = var.environment
      AGENTPAY_REPOSITORY_MODE                      = "dynamodb"
      AGENTPAY_WEB_ORIGIN                           = var.web_origin
      AGENTPAY_API_ORIGIN                           = aws_apigatewayv2_api.http[0].api_endpoint
      AGENTPAY_PUBLIC_BASE_URL                      = aws_apigatewayv2_api.http[0].api_endpoint
      AGENTPAY_TABLE_NAME                           = var.table_name
      AGENTPAY_EVIDENCE_BUCKET                      = var.evidence_bucket_name
      AGENTPAY_EVIDENCE_KMS_KEY_ID                  = var.evidence_kms_key_id
      AGENTPAY_CAPABILITY_SIGNING_KEY_ID            = var.capability_signing_key_id
      AGENTPAY_CAPABILITY_VERIFICATION_KEY_IDS      = jsonencode(var.capability_verification_key_ids)
      AGENTPAY_APPLICATION_SECRETS_KMS_KEY_ID       = var.application_secrets_kms_key_id
      AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN         = var.credential_pepper_secret_arn
      AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN = var.confirmation_grant_pepper_secret_arn
    }
  }

  depends_on = [
    aws_cloudwatch_log_group.lambda,
    aws_iam_role_policy.runtime_logs,
  ]
}

resource "aws_apigatewayv2_integration" "api" {
  count = var.deployment_enabled ? 1 : 0

  api_id                 = aws_apigatewayv2_api.http[0].id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.api[0].invoke_arn
  integration_method     = "POST"
  payload_format_version = "2.0"
  timeout_milliseconds   = var.lambda_timeout_seconds * 1000
}

resource "aws_apigatewayv2_route" "default" {
  count = var.deployment_enabled ? 1 : 0

  api_id    = aws_apigatewayv2_api.http[0].id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.api[0].id}"
}

resource "aws_apigatewayv2_stage" "default" {
  count = var.deployment_enabled ? 1 : 0

  api_id      = aws_apigatewayv2_api.http[0].id
  name        = "$default"
  auto_deploy = true

  default_route_settings {
    detailed_metrics_enabled = true
    throttling_burst_limit   = var.throttling_burst_limit
    throttling_rate_limit    = var.throttling_rate_limit
  }

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api[0].arn
    format = jsonencode({
      requestId        = "$context.requestId"
      requestTime      = "$context.requestTime"
      httpMethod       = "$context.httpMethod"
      routeKey         = "$context.routeKey"
      status           = "$context.status"
      responseLength   = "$context.responseLength"
      integrationError = "$context.integrationErrorMessage"
    })
  }
}

resource "aws_lambda_permission" "http_api" {
  count = var.deployment_enabled ? 1 : 0

  statement_id  = "AllowHTTPAPIInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api[0].function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.http[0].execution_arn}/*/*"
}
