output "vpc_id" {
  value = aws_vpc.main.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  value = aws_subnet.private[*].id
}

output "sg_alb_public_id" {
  value = aws_security_group.alb_public.id
}

output "sg_ecs_tasks_id" {
  value = aws_security_group.ecs_tasks.id
}

output "sg_redis_id" {
  value = aws_security_group.redis.id
}

output "sg_alb_stub_origin_id" {
  value = aws_security_group.alb_stub_origin.id
}
