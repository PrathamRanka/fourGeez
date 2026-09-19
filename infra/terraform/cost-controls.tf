locals {
  budget_notifications = [
    {
      notification_type = "ACTUAL"
      threshold         = 50
    },
    {
      notification_type = "ACTUAL"
      threshold         = 80
    },
    {
      notification_type = "ACTUAL"
      threshold         = 100
    },
    {
      notification_type = "FORECASTED"
      threshold         = 100
    },
  ]
}

resource "aws_budgets_budget" "monthly" {
  count = var.budget_alert_email == null ? 0 : 1

  name         = "${var.project_name}-${var.environment}-monthly-cost"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_limit_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  dynamic "notification" {
    for_each = local.budget_notifications

    content {
      comparison_operator        = "GREATER_THAN"
      notification_type          = notification.value.notification_type
      threshold                  = notification.value.threshold
      threshold_type             = "PERCENTAGE"
      subscriber_email_addresses = [coalesce(var.budget_alert_email, "invalid@example.invalid")]
    }
  }
}
