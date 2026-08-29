resource "aws_sqs_queue" "admission_events" {
  name                        = "${var.environment}-admission-events.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
  visibility_timeout_seconds  = 30
  message_retention_seconds   = 86400

  tags = { Environment = var.environment }
}
