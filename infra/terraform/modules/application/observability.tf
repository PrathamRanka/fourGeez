locals {
  operational_metric_filters = {
    mcp_failure = {
      metric_name = "MCPFailures"
      pattern     = "{ $.operationalEvent = \"mcp_failure\" }"
    }
    checkout_failure = {
      metric_name = "CheckoutFailures"
      pattern     = "{ $.operationalEvent = \"checkout_failure\" }"
    }
    facilitator_failure = {
      metric_name = "FacilitatorFailures"
      pattern     = "{ $.operationalEvent = \"facilitator_failure\" }"
    }
    evidence_failure = {
      metric_name = "EvidenceFailures"
      pattern     = "{ $.operationalEvent = \"evidence_failure\" }"
    }
    seller_forwarding_failure = {
      metric_name = "SellerForwardingFailures"
      pattern     = "{ $.operationalEvent = \"seller_forwarding_failure\" }"
    }
    payment_replay = {
      metric_name = "PaymentReplays"
      pattern     = "{ $.operationalEvent = \"payment_replay\" }"
    }
    webhook_retry_scheduled = {
      metric_name = "WebhookRetries"
      pattern     = "{ $.operationalEvent = \"webhook_retry_scheduled\" }"
    }
    webhook_dead_letter = {
      metric_name = "WebhookDeadLetters"
      pattern     = "{ $.operationalEvent = \"webhook_dead_letter\" }"
    }
  }
}

resource "aws_cloudwatch_log_metric_filter" "operational" {
  for_each = var.deployment_enabled ? local.operational_metric_filters : {}

  name           = "${local.resource_prefix}-${replace(each.key, "_", "-")}"
  pattern        = each.value.pattern
  log_group_name = aws_cloudwatch_log_group.lambda[0].name

  metric_transformation {
    name      = each.value.metric_name
    namespace = "AgentPay/Operations"
    value     = "1"
    unit      = "Count"
  }
}

resource "aws_cloudwatch_metric_alarm" "operational" {
  for_each = var.deployment_enabled ? local.operational_metric_filters : {}

  alarm_name          = "${local.resource_prefix}-${replace(each.key, "_", "-")}"
  alarm_description   = "AgentPay ${each.key} detected in the seller production path."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  datapoints_to_alarm = 1
  threshold           = 1
  metric_name         = each.value.metric_name
  namespace           = "AgentPay/Operations"
  period              = 300
  statistic           = "Sum"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  depends_on = [aws_cloudwatch_log_metric_filter.operational]
}

resource "aws_cloudwatch_metric_alarm" "api_server_errors" {
  count = var.deployment_enabled ? 1 : 0

  alarm_name          = "${local.resource_prefix}-api-5xx"
  alarm_description   = "The AgentPay HTTP API returned one or more 5xx responses in five minutes."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  datapoints_to_alarm = 1
  threshold           = 1
  metric_name         = "5xx"
  namespace           = "AWS/ApiGateway"
  period              = 300
  statistic           = "Sum"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  dimensions = {
    ApiId = aws_apigatewayv2_api.http[0].id
    Stage = aws_apigatewayv2_stage.default[0].name
  }
}

resource "aws_cloudwatch_metric_alarm" "api_latency" {
  count = var.deployment_enabled ? 1 : 0

  alarm_name          = "${local.resource_prefix}-api-p95-latency"
  alarm_description   = "The AgentPay HTTP API p95 latency exceeded the pilot threshold."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  datapoints_to_alarm = 2
  threshold           = var.api_p95_latency_alarm_ms
  metric_name         = "Latency"
  namespace           = "AWS/ApiGateway"
  period              = 300
  extended_statistic  = "p95"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  dimensions = {
    ApiId = aws_apigatewayv2_api.http[0].id
    Stage = aws_apigatewayv2_stage.default[0].name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  count = var.deployment_enabled ? 1 : 0

  alarm_name          = "${local.resource_prefix}-lambda-errors"
  alarm_description   = "The AgentPay API Lambda returned one or more invocation errors."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  datapoints_to_alarm = 1
  threshold           = 1
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  dimensions = {
    FunctionName = aws_lambda_function.api[0].function_name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_throttles" {
  count = var.deployment_enabled ? 1 : 0

  alarm_name          = "${local.resource_prefix}-lambda-throttles"
  alarm_description   = "The AgentPay API Lambda was throttled."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  datapoints_to_alarm = 1
  threshold           = 1
  metric_name         = "Throttles"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  dimensions = {
    FunctionName = aws_lambda_function.api[0].function_name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_duration" {
  count = var.deployment_enabled ? 1 : 0

  alarm_name          = "${local.resource_prefix}-lambda-p95-duration"
  alarm_description   = "The AgentPay API Lambda p95 duration is approaching its timeout."
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  datapoints_to_alarm = 2
  threshold           = var.lambda_p95_duration_alarm_ms
  metric_name         = "Duration"
  namespace           = "AWS/Lambda"
  period              = 300
  extended_statistic  = "p95"
  treat_missing_data  = "notBreaching"
  alarm_actions       = var.alarm_action_arns
  ok_actions          = var.alarm_action_arns

  dimensions = {
    FunctionName = aws_lambda_function.api[0].function_name
  }
}

resource "aws_cloudwatch_dashboard" "operations" {
  count = var.deployment_enabled ? 1 : 0

  dashboard_name = "${local.resource_prefix}-seller-operations"
  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "HTTP API requests, errors, and latency"
          region = data.aws_region.current.region
          view   = "timeSeries"
          metrics = [
            ["AWS/ApiGateway", "Count", "ApiId", aws_apigatewayv2_api.http[0].id, "Stage", aws_apigatewayv2_stage.default[0].name, { stat = "Sum" }],
            [".", "4xx", ".", ".", ".", ".", { stat = "Sum" }],
            [".", "5xx", ".", ".", ".", ".", { stat = "Sum" }],
            [".", "Latency", ".", ".", ".", ".", { stat = "p95", yAxis = "right" }],
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Lambda errors, throttles, and duration"
          region = data.aws_region.current.region
          view   = "timeSeries"
          metrics = [
            ["AWS/Lambda", "Errors", "FunctionName", aws_lambda_function.api[0].function_name, { stat = "Sum" }],
            [".", "Throttles", ".", ".", { stat = "Sum" }],
            [".", "Duration", ".", ".", { stat = "p95", yAxis = "right" }],
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 24
        height = 7
        properties = {
          title   = "Seller-path operational failures"
          region  = data.aws_region.current.region
          view    = "timeSeries"
          stacked = false
          metrics = [for event in values(local.operational_metric_filters) : ["AgentPay/Operations", event.metric_name, { stat = "Sum" }]]
        }
      },
    ]
  })
}

data "aws_region" "current" {}
