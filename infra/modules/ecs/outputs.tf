output "ecr_queueserver_url" {
  value = aws_ecr_repository.queueserver.repository_url
}

output "ecr_scheduler_url" {
  value = aws_ecr_repository.scheduler.repository_url
}

output "ecr_stuborigin_url" {
  value = aws_ecr_repository.stuborigin.repository_url
}

output "alb_queue_api_dns" { value = aws_lb.alb_queue_api.dns_name }
output "alb_stub_origin_dns" { value = aws_lb.alb_stub_origin.dns_name }
output "ecs_cluster_name" { value = aws_ecs_cluster.main.name }
output "ecs_queueserver_service_name" { value = aws_ecs_service.queueserver.name }
output "ecs_scheduler_service_name" { value = aws_ecs_service.scheduler.name }
output "ecs_stuborigin_service_name" { value = aws_ecs_service.stuborigin.name }
