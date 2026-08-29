resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.environment}-redis"
  subnet_ids = var.private_subnet_ids
}

resource "aws_elasticache_replication_group" "redis_queue" {
  replication_group_id       = "${var.environment}-redis-queue"
  description                 = "Queue server and scheduler Redis"
  node_type                   = var.redis_node_type
  num_cache_clusters          = 1
  port                        = 6379
  subnet_group_name           = aws_elasticache_subnet_group.redis.name
  security_group_ids          = [var.sg_redis_id]
  at_rest_encryption_enabled  = true
  transit_encryption_enabled  = true
  automatic_failover_enabled  = false
}

resource "aws_elasticache_replication_group" "redis_origin" {
  replication_group_id       = "${var.environment}-redis-origin"
  description                 = "Stub origin Redis"
  node_type                   = var.redis_node_type
  num_cache_clusters          = 1
  port                        = 6379
  subnet_group_name           = aws_elasticache_subnet_group.redis.name
  security_group_ids          = [var.sg_redis_id]
  at_rest_encryption_enabled  = true
  transit_encryption_enabled  = true
  automatic_failover_enabled  = false
}
