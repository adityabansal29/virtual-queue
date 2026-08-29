output "redis_queue_primary_endpoint" {
  value = "${aws_elasticache_replication_group.redis_queue.primary_endpoint_address}:6379"
}

output "redis_origin_primary_endpoint" {
  value = "${aws_elasticache_replication_group.redis_origin.primary_endpoint_address}:6379"
}
