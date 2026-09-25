locals {
  resource_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_dynamodb_table" "agentpay" {
  name         = "${local.resource_prefix}-main"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  deletion_protection_enabled = var.environment != "dev"
  stream_enabled              = true
  stream_view_type            = "NEW_IMAGE"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  attribute {
    name = "GSI1PK"
    type = "S"
  }

  attribute {
    name = "GSI1SK"
    type = "S"
  }

  attribute {
    name = "GSI2PK"
    type = "S"
  }

  attribute {
    name = "GSI2SK"
    type = "S"
  }

  attribute {
    name = "GSI3PK"
    type = "S"
  }

  attribute {
    name = "GSI3SK"
    type = "S"
  }

  attribute {
    name = "GSI4PK"
    type = "S"
  }

  attribute {
    name = "GSI4SK"
    type = "S"
  }

  global_secondary_index {
    name = "GSI1"

    key_schema {
      attribute_name = "GSI1PK"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "GSI1SK"
      key_type       = "RANGE"
    }

    projection_type = "ALL"
  }

  global_secondary_index {
    name = "GSI2"

    key_schema {
      attribute_name = "GSI2PK"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "GSI2SK"
      key_type       = "RANGE"
    }

    projection_type = "ALL"
  }

  global_secondary_index {
    name = "GSI3"

    key_schema {
      attribute_name = "GSI3PK"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "GSI3SK"
      key_type       = "RANGE"
    }

    projection_type = "ALL"
  }

  global_secondary_index {
    name = "GSI4"

    key_schema {
      attribute_name = "GSI4PK"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "GSI4SK"
      key_type       = "RANGE"
    }

    projection_type = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }
}

resource "aws_kms_key" "evidence_signing" {
  description              = "AgentPay ${var.environment} evidence signing"
  key_usage                = "SIGN_VERIFY"
  customer_master_key_spec = "ECC_NIST_P256"
  deletion_window_in_days  = 30

  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_kms_alias" "evidence_signing" {
  name          = "alias/${local.resource_prefix}-evidence-signing"
  target_key_id = aws_kms_key.evidence_signing.key_id
}

resource "aws_s3_bucket" "evidence" {
  bucket              = "${local.resource_prefix}-evidence-${var.aws_account_id}-${var.aws_region}"
  object_lock_enabled = true
  force_destroy       = true

  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_s3_bucket_ownership_controls" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_public_access_block" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "aws:kms"
    }

    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_object_lock_configuration" "evidence" {
  bucket = aws_s3_bucket.evidence.id

  rule {
    default_retention {
      mode = "GOVERNANCE"
      days = var.evidence_retention_days
    }
  }

  depends_on = [aws_s3_bucket_versioning.evidence]
}

data "aws_iam_policy_document" "evidence" {
  statement {
    sid    = "DenyInsecureTransport"
    effect = "Deny"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions   = ["s3:*"]
    resources = [aws_s3_bucket.evidence.arn, "${aws_s3_bucket.evidence.arn}/*"]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "evidence" {
  bucket = aws_s3_bucket.evidence.id
  policy = data.aws_iam_policy_document.evidence.json

  depends_on = [aws_s3_bucket_public_access_block.evidence]
}
