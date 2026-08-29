resource "aws_appautoscaling_target" "queueserver" {
  max_capacity = 10
  min_capacity = 1
  resource_id = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.queueserver.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace = "ecs"
}

resource "aws_appautoscaling_policy" "queueserver_cpu" {
  name = "${var.environment}-queueserver-cpu-scaling"
  policy_type = "TargetTrackingScaling"
  resource_id = aws_appautoscaling_target.queueserver.resource_id
  scalable_dimension = aws_appautoscaling_target.queueserver.scalable_dimension
  service_namespace = aws_appautoscaling_target.queueserver.service_namespace
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification { predefined_metric_type = "ECSServiceAverageCPUUtilization" }
    target_value = 60.0
    scale_in_cooldown = 300
    scale_out_cooldown = 60
  }
}
