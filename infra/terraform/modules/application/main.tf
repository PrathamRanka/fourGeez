locals {
  resource_prefix       = "${var.project_name}-${var.environment}"
  lambda_log_group_name = "/aws/lambda/${local.resource_prefix}-api"
  seller_authenticated_routes = toset([
    "ANY /v1/me/{proxy+}",
    "ANY /v1/sellers/{sellerId}/{proxy+}",
    "POST /v1/sellers",
  ])
}

data "aws_servicequotas_service_quota" "lambda_concurrency" {
  count = var.deployment_enabled ? 1 : 0

  service_code = "lambda"
  quota_code   = "L-B99A9384"
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

resource "aws_sqs_queue" "publication_outbox_dlq" {
  count = var.deployment_enabled ? 1 : 0

  name                       = "${local.resource_prefix}-publication-outbox-dlq"
  message_retention_seconds  = 1209600
  visibility_timeout_seconds = 60
  sqs_managed_sse_enabled    = true
}

data "aws_iam_policy_document" "publication_outbox" {
  count = var.deployment_enabled ? 1 : 0

  statement {
    sid = "ReadPublicationStream"
    actions = [
      "dynamodb:DescribeStream",
      "dynamodb:GetRecords",
      "dynamodb:GetShardIterator",
    ]
    resources = [var.table_stream_arn]
  }

  statement {
    sid       = "ListPublicationStreams"
    actions   = ["dynamodb:ListStreams"]
    resources = ["*"]
  }

  statement {
    sid       = "SendPublicationFailures"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.publication_outbox_dlq[0].arn]
  }
}

resource "aws_iam_role_policy" "publication_outbox" {
  count = var.deployment_enabled ? 1 : 0

  name   = "${local.resource_prefix}-publication-outbox"
  role   = var.api_runtime_role_name
  policy = data.aws_iam_policy_document.publication_outbox[0].json
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
      "x-agentpay-agent-key",
      "x-agentpay-project-key",
      "payment-signature",
      "x-payment",
    ]
    allow_methods  = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
    allow_origins  = [var.web_origin]
    expose_headers = ["payment-required", "payment-response", "x-agentpay-request-id"]
    max_age        = 300
  }
}

resource "aws_apigatewayv2_authorizer" "seller" {
  count = var.deployment_enabled ? 1 : 0

  api_id           = aws_apigatewayv2_api.http[0].id
  authorizer_type  = "JWT"
  identity_sources = ["$request.header.Authorization"]
  name             = "${local.resource_prefix}-seller"

  jwt_configuration {
    audience = [var.seller_user_pool_client_id]
    issuer   = var.seller_identity_issuer
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
      AGENTPAY_API_ORIGIN                           = var.public_api_origin
      AGENTPAY_PUBLIC_BASE_URL                      = var.public_api_origin
      AGENTPAY_PAYMENT_MODE                         = var.payment_mode
      AGENTPAY_TABLE_NAME                           = var.table_name
      AGENTPAY_EVIDENCE_BUCKET                      = var.evidence_bucket_name
      AGENTPAY_EVIDENCE_KMS_KEY_ID                  = var.evidence_kms_key_id
      AGENTPAY_CAPABILITY_SIGNING_KEY_ID            = var.capability_signing_key_id
      AGENTPAY_CAPABILITY_VERIFICATION_KEY_IDS      = jsonencode(var.capability_verification_key_ids)
      AGENTPAY_APPLICATION_SECRETS_KMS_KEY_ID       = var.application_secrets_kms_key_id
      AGENTPAY_CREDENTIAL_PEPPER_SECRET_ARN         = var.credential_pepper_secret_arn
      AGENTPAY_CONFIRMATION_GRANT_PEPPER_SECRET_ARN = var.confirmation_grant_pepper_secret_arn
      AGENTPAY_SELLER_USER_POOL_ID                  = var.seller_user_pool_id
      AGENTPAY_SELLER_USER_POOL_CLIENT_ID           = var.seller_user_pool_client_id
      AGENTPAY_FACILITATOR_URL                      = var.facilitator_url
      AGENTPAY_X402_NETWORK                         = var.x402_network
      AGENTPAY_X402_ASSET                           = var.x402_asset
    }
  }

  logging_config {
    application_log_level = "INFO"
    log_format            = "JSON"
    system_log_level      = "WARN"
  }

  lifecycle {
    precondition {
      condition     = data.aws_servicequotas_service_quota.lambda_concurrency[0].value > 10 + var.lambda_reserved_concurrency
      error_message = "The regional Lambda concurrency quota must exceed 10 plus api_reserved_concurrency before deployment."
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

resource "aws_apigatewayv2_route" "seller_authenticated" {
  for_each = var.deployment_enabled ? local.seller_authenticated_routes : toset([])

  api_id             = aws_apigatewayv2_api.http[0].id
  route_key          = each.value
  target             = "integrations/${aws_apigatewayv2_integration.api[0].id}"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.seller[0].id
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
      requestId               = "$context.requestId"
      requestTime             = "$context.requestTime"
      httpMethod              = "$context.httpMethod"
      path                    = "$context.path"
      routeKey                = "$context.routeKey"
      status                  = "$context.status"
      integrationLatency      = "$context.integrationLatency"
      integrationErrorMessage = "$context.integrationErrorMessage"
      responseLatency         = "$context.responseLatency"
      responseLength          = "$context.responseLength"
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

resource "aws_lambda_event_source_mapping" "publication_outbox" {
  count = var.deployment_enabled ? 1 : 0

  event_source_arn                   = var.table_stream_arn
  function_name                      = aws_lambda_function.api[0].arn
  starting_position                  = "LATEST"
  batch_size                         = 10
  maximum_batching_window_in_seconds = 1
  maximum_record_age_in_seconds      = 3600
  maximum_retry_attempts             = 3
  bisect_batch_on_function_error     = true
  function_response_types            = ["ReportBatchItemFailures"]

  filter_criteria {
    filter {
      pattern = jsonencode({
        eventName = ["INSERT"]
        dynamodb = {
          NewImage = {
            entity = { S = ["publicationOutbox"] }
          }
        }
      })
    }
  }

  destination_config {
    on_failure {
      destination_arn = aws_sqs_queue.publication_outbox_dlq[0].arn
    }
  }

  depends_on = [aws_iam_role_policy.publication_outbox]
}
