resource "aws_kms_key" "application_secrets" {
  description             = "AgentPay ${var.environment} secret and replay-envelope encryption"
  key_usage               = "ENCRYPT_DECRYPT"
  enable_key_rotation     = true
  deletion_window_in_days = 30

  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_kms_alias" "application_secrets" {
  name          = "alias/${local.resource_prefix}-application-secrets"
  target_key_id = aws_kms_key.application_secrets.key_id
}

resource "aws_secretsmanager_secret" "credential_pepper" {
  name                    = "${var.project_name}/${var.environment}/credential-pepper"
  description             = "HMAC pepper for AgentPay project credential digests"
  kms_key_id              = aws_kms_key.application_secrets.arn
  recovery_window_in_days = 30
}

resource "aws_secretsmanager_secret" "confirmation_grant_pepper" {
  name                    = "${var.project_name}/${var.environment}/confirmation-grant-pepper"
  description             = "HMAC pepper for one-time MCP confirmation grant digests"
  kms_key_id              = aws_kms_key.application_secrets.arn
  recovery_window_in_days = 30
}

resource "aws_kms_key" "capability_signing" {
  for_each = var.capability_signing_key_versions

  description              = "AgentPay ${var.environment} ES256 capability signing ${each.key}"
  key_usage                = "SIGN_VERIFY"
  customer_master_key_spec = "ECC_NIST_P256"
  deletion_window_in_days  = 30

  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_kms_alias" "capability_signing_version" {
  for_each = var.capability_signing_key_versions

  name          = "alias/${local.resource_prefix}-capability-${each.key}"
  target_key_id = aws_kms_key.capability_signing[each.key].key_id
}

resource "aws_kms_alias" "capability_signing_current" {
  name          = "alias/${local.resource_prefix}-capability-current"
  target_key_id = aws_kms_key.capability_signing[var.active_capability_signing_key_version].key_id
}

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "api_runtime" {
  name               = "${local.resource_prefix}-api-runtime"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "api_runtime" {
  statement {
    sid = "DynamoDBApplicationData"
    actions = [
      "dynamodb:BatchGetItem",
      "dynamodb:BatchWriteItem",
      "dynamodb:ConditionCheckItem",
      "dynamodb:DescribeTable",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:Query",
      "dynamodb:TransactGetItems",
      "dynamodb:TransactWriteItems",
      "dynamodb:UpdateItem",
    ]
    resources = [
      aws_dynamodb_table.agentpay.arn,
      "${aws_dynamodb_table.agentpay.arn}/index/*",
    ]
  }

  statement {
    sid       = "EvidenceBucketMetadata"
    actions   = ["s3:GetBucketLocation", "s3:ListBucket"]
    resources = [aws_s3_bucket.evidence.arn]
  }

  statement {
    sid       = "EvidenceObjects"
    actions   = ["s3:GetObject", "s3:PutObject"]
    resources = ["${aws_s3_bucket.evidence.arn}/*"]
  }

  statement {
    sid       = "EvidenceSigning"
    actions   = ["kms:GetPublicKey", "kms:Sign", "kms:Verify"]
    resources = [aws_kms_key.evidence_signing.arn]
  }

  statement {
    sid       = "CapabilitySigning"
    actions   = ["kms:GetPublicKey", "kms:Sign", "kms:Verify"]
    resources = [for key in aws_kms_key.capability_signing : key.arn]
  }

  statement {
    sid = "ApplicationEnvelopeEncryption"
    actions = [
      "kms:Decrypt",
      "kms:DescribeKey",
      "kms:Encrypt",
      "kms:GenerateDataKey",
    ]
    resources = [aws_kms_key.application_secrets.arn]
  }

  statement {
    sid     = "PepperSecretRead"
    actions = ["secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue"]
    resources = [
      aws_secretsmanager_secret.credential_pepper.arn,
      aws_secretsmanager_secret.confirmation_grant_pepper.arn,
    ]
  }

  statement {
    sid = "SellerSecretLifecycle"
    actions = [
      "secretsmanager:CreateSecret",
      "secretsmanager:DescribeSecret",
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:TagResource",
    ]
    resources = [
      "arn:aws:secretsmanager:${var.aws_region}:${var.aws_account_id}:secret:${var.project_name}/${var.environment}/sellers/*",
    ]
  }
}

resource "aws_iam_role_policy" "api_runtime" {
  name   = "${local.resource_prefix}-api-runtime"
  role   = aws_iam_role.api_runtime.id
  policy = data.aws_iam_policy_document.api_runtime.json
}

resource "aws_iam_role" "evidence_verifier" {
  name               = "${local.resource_prefix}-evidence-verifier"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

data "aws_iam_policy_document" "evidence_verifier" {
  statement {
    sid       = "EvidenceBucketRead"
    actions   = ["s3:GetBucketLocation", "s3:ListBucket"]
    resources = [aws_s3_bucket.evidence.arn]
  }

  statement {
    sid       = "EvidenceObjectRead"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.evidence.arn}/*"]
  }

  statement {
    sid       = "EvidenceSignatureVerification"
    actions   = ["kms:GetPublicKey", "kms:Verify"]
    resources = [aws_kms_key.evidence_signing.arn]
  }
}

resource "aws_iam_role_policy" "evidence_verifier" {
  name   = "${local.resource_prefix}-evidence-verifier"
  role   = aws_iam_role.evidence_verifier.id
  policy = data.aws_iam_policy_document.evidence_verifier.json
}
