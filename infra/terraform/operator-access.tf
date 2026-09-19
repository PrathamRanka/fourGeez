data "aws_iam_policy_document" "launch_entitlement_operator_assume" {
  count = length(var.launch_entitlement_operator_principal_arns) > 0 ? 1 : 0

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = var.launch_entitlement_operator_principal_arns
    }
  }
}

resource "aws_iam_role" "launch_entitlement_operator" {
  count = length(var.launch_entitlement_operator_principal_arns) > 0 ? 1 : 0

  name               = "${var.project_name}-${var.environment}-launch-entitlement-operator"
  assume_role_policy = data.aws_iam_policy_document.launch_entitlement_operator_assume[0].json
  tags               = local.common_tags
}

data "aws_iam_policy_document" "launch_entitlement_operator" {
  count = length(var.launch_entitlement_operator_principal_arns) > 0 ? 1 : 0

  statement {
    sid = "ReadAndAtomicallyChangeSellerEntitlement"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:TransactWriteItems",
    ]
    resources = [module.foundation.table_arn]

    condition {
      test     = "ForAllValues:StringLike"
      variable = "dynamodb:LeadingKeys"
      values   = ["SELLER#*"]
    }
  }
}

resource "aws_iam_role_policy" "launch_entitlement_operator" {
  count = length(var.launch_entitlement_operator_principal_arns) > 0 ? 1 : 0

  name   = "${var.project_name}-${var.environment}-launch-entitlement-operator"
  role   = aws_iam_role.launch_entitlement_operator[0].id
  policy = data.aws_iam_policy_document.launch_entitlement_operator[0].json
}
