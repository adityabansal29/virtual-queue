data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task_execution" {
  name               = "${var.environment}-ecs-task-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
  inline_policy {
    name = "read-secrets"
    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Effect = "Allow", Action = ["ssm:GetParameters"],
        Resource = [var.ssm_admission_secret_arn, var.ssm_session_secret_arn]
      }]
    })
  }
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "ecs_task_role" {
  name               = "${var.environment}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
  inline_policy {
    name = "application-access"
    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [
        { Effect = "Allow", Action = ["dynamodb:PutItem"], Resource = [var.dynamodb_sessions_table_arn, var.dynamodb_events_table_arn, var.dynamodb_audit_log_table_arn] },
        { Effect = "Allow", Action = ["sqs:SendMessage"], Resource = [var.sqs_admission_queue_arn] },
        { Effect = "Allow", Action = ["s3:HeadObject", "s3:GetObject", "s3:PutObject"], Resource = [var.queue_page_bucket_arn] }
      ]
    })
  }
}

resource "aws_ecs_cluster" "main" {
  name = "${var.environment}-queue-cluster"
}

resource "aws_cloudwatch_log_group" "queueserver" {
  name              = "/ecs/${var.environment}/queueserver"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "scheduler" {
  name              = "/ecs/${var.environment}/scheduler"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "stuborigin" {
  name              = "/ecs/${var.environment}/stuborigin"
  retention_in_days = 7
}

locals {
  log_options = { "awslogs-region" = var.aws_region, "awslogs-stream-prefix" = "ecs" }
}

resource "aws_ecs_task_definition" "queueserver" {
  family                   = "${var.environment}-queueserver"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions = jsonencode([{
    name = "queueserver", image = "${aws_ecr_repository.queueserver.repository_url}:latest", essential = true,
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "REDIS_ADDR", value = var.redis_queue_addr }, { name = "PORT", value = "8080" },
      { name = "AWS_REGION", value = var.aws_region }, { name = "QUEUE_PAGE_URL", value = var.queue_page_url },
      { name = "QUEUE_PAGE_BUCKET_NAME", value = var.queue_page_bucket_name },
      { name = "CORS_ALLOWED_ORIGINS", value = var.cors_allowed_origins }
    ]
    logConfiguration = { logDriver = "awslogs", options = merge(local.log_options, { "awslogs-group" = aws_cloudwatch_log_group.queueserver.name }) }
  }])
}

resource "aws_ecs_task_definition" "scheduler" {
  family                   = "${var.environment}-scheduler"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions = jsonencode([{
    name = "scheduler", image = "${aws_ecr_repository.scheduler.repository_url}:latest", essential = true
    environment = [
      { name = "REDIS_ADDR", value = var.redis_queue_addr }, { name = "AWS_REGION", value = var.aws_region },
      { name = "DYNAMO_SESSIONS_TABLE", value = "${var.environment}-queue-sessions" }, { name = "SQS_ADMISSION_QUEUE_URL", value = var.sqs_admission_queue_url }
    ]
    secrets = [{ name = "ADMISSION_SECRET", valueFrom = var.ssm_admission_secret_arn }]
    logConfiguration = { logDriver = "awslogs", options = merge(local.log_options, { "awslogs-group" = aws_cloudwatch_log_group.scheduler.name }) }
  }])
}

resource "aws_ecs_task_definition" "stuborigin" {
  family                   = "${var.environment}-stuborigin"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  container_definitions = jsonencode([{
    name = "stuborigin", image = "${aws_ecr_repository.stuborigin.repository_url}:latest", essential = true,
    portMappings = [{ containerPort = 8081, protocol = "tcp" }]
    environment = [
      { name = "REDIS_ADDR", value = var.redis_origin_addr }, { name = "AWS_REGION", value = var.aws_region },
      { name = "SECURE", value = "true" }, { name = "EVENT_ID", value = var.event_id }, { name = "QUEUE_JOIN_URL", value = var.queue_join_url }
    ]
    secrets = [
      { name = "ADMISSION_SECRET", valueFrom = var.ssm_admission_secret_arn }, { name = "SESSION_SECRET", valueFrom = var.ssm_session_secret_arn }
    ]
    logConfiguration = { logDriver = "awslogs", options = merge(local.log_options, { "awslogs-group" = aws_cloudwatch_log_group.stuborigin.name }) }
  }])
}

resource "aws_lb" "alb_queue_api" {
  name = "${var.environment}-queue-api"
  internal = false
  load_balancer_type = "application"
  security_groups = [var.sg_alb_public_id]
  subnets = var.public_subnet_ids
}

resource "aws_lb_target_group" "queue_api" {
  name = "${var.environment}-queue-api"
  port = 8080
  protocol = "HTTP"
  target_type = "ip"
  vpc_id = var.vpc_id
  health_check { path = "/health" }
}

resource "aws_lb_listener" "queue_api" {
  load_balancer_arn = aws_lb.alb_queue_api.arn
  port = 80
  protocol = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.queue_api.arn
  }
}

resource "aws_lb" "alb_stub_origin" {
  name = "${var.environment}-stub-origin"
  internal = false
  load_balancer_type = "application"
  security_groups = [var.sg_alb_stub_origin_id]
  subnets = var.public_subnet_ids
}

resource "aws_lb_target_group" "stub_origin" {
  name = "${var.environment}-stub-origin"
  port = 8081
  protocol = "HTTP"
  target_type = "ip"
  vpc_id = var.vpc_id
  health_check { path = "/health" }
}

resource "aws_lb_listener" "stub_origin" {
  load_balancer_arn = aws_lb.alb_stub_origin.arn
  port = 80
  protocol = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.stub_origin.arn
  }
}

resource "aws_ecs_service" "queueserver" {
  name = "${var.environment}-queueserver"
  cluster = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.queueserver.arn
  desired_count = 1
  launch_type = "FARGATE"
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.sg_ecs_tasks_id]
    assign_public_ip = false
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.queue_api.arn
    container_name   = "queueserver"
    container_port   = 8080
  }
}

resource "aws_ecs_service" "scheduler" {
  name = "${var.environment}-scheduler"
  cluster = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.scheduler.arn
  desired_count = 1
  launch_type = "FARGATE"
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.sg_ecs_tasks_id]
    assign_public_ip = false
  }
}

resource "aws_ecs_service" "stuborigin" {
  name = "${var.environment}-stuborigin"
  cluster = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.stuborigin.arn
  desired_count = 1
  launch_type = "FARGATE"
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.sg_ecs_tasks_id]
    assign_public_ip = false
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.stub_origin.arn
    container_name   = "stuborigin"
    container_port   = 8081
  }
}
