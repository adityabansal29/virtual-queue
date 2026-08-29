output "queue_sessions_table_arn" {
  value = aws_dynamodb_table.queue_sessions.arn
}

output "queue_sessions_table_name" {
  value = aws_dynamodb_table.queue_sessions.name
}

output "queue_events_table_arn" {
  value = aws_dynamodb_table.queue_events.arn
}

output "queue_events_table_name" {
  value = aws_dynamodb_table.queue_events.name
}

output "queue_audit_log_table_arn" {
  value = aws_dynamodb_table.queue_audit_log.arn
}

output "queue_audit_log_table_name" {
  value = aws_dynamodb_table.queue_audit_log.name
}
