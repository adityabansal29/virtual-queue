output "admission_events_queue_url" {
  value = aws_sqs_queue.admission_events.url
}

output "admission_events_queue_arn" {
  value = aws_sqs_queue.admission_events.arn
}
