locals {
  resource_prefix = "${var.project_name}-${var.environment}"
}

resource "aws_dynamodb_table" "agentpay" {
  name         = "${local.resource_prefix}-main"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "PK"
  range_key    = "SK"

  deletion_protection_enabled = var.environment != "dev"

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
