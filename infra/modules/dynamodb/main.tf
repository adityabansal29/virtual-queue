resource "aws_dynamodb_table" "queue_sessions" {
  name         = "${var.environment}-queue-sessions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "ticketId"

  attribute {
    name = "ticketId"
    type = "S"
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = { Environment = var.environment }
}

resource "aws_dynamodb_table" "queue_events" {
  name         = "${var.environment}-queue-events"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "eventId"

  attribute {
    name = "eventId"
    type = "S"
  }

  tags = { Environment = var.environment }
}

resource "aws_dynamodb_table" "queue_audit_log" {
  name         = "${var.environment}-queue-audit-log"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "eventId"
  range_key    = "timestamp"

  attribute {
    name = "eventId"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "S"
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = { Environment = var.environment }
}
