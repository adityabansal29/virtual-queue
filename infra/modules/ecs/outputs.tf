output "ecr_queueserver_url" {
  value = aws_ecr_repository.queueserver.repository_url
}

output "ecr_scheduler_url" {
  value = aws_ecr_repository.scheduler.repository_url
}

output "ecr_stuborigin_url" {
  value = aws_ecr_repository.stuborigin.repository_url
}
