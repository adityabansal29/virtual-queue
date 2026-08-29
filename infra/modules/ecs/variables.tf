variable "environment" {
  type    = string
  default = "dev"
}

variable "aws_region" {
  type    = string
  default = "ap-south-1"
}

variable "vpc_id" {
  type    = string
  default = ""
}

variable "private_subnet_ids" {
  type    = list(string)
  default = []
}

variable "public_subnet_ids" {
  type    = list(string)
  default = []
}

variable "sg_ecs_tasks_id" {
  type    = string
  default = ""
}

variable "sg_alb_public_id" {
  type    = string
  default = ""
}

variable "sg_alb_stub_origin_id" {
  type    = string
  default = ""
}

variable "ecr_queueserver_url" {
  type    = string
  default = ""
}

variable "ecr_scheduler_url" {
  type    = string
  default = ""
}

variable "ecr_stuborigin_url" {
  type    = string
  default = ""
}

variable "redis_queue_addr" {
  type    = string
  default = ""
}

variable "redis_origin_addr" {
  type    = string
  default = ""
}

variable "ssm_admission_secret_arn" {
  type    = string
  default = ""
}

variable "ssm_session_secret_arn" {
  type    = string
  default = ""
}

variable "dynamodb_sessions_table_arn" {
  type    = string
  default = ""
}

variable "dynamodb_events_table_arn" {
  type    = string
  default = ""
}

variable "dynamodb_audit_log_table_arn" {
  type    = string
  default = ""
}

variable "sqs_admission_queue_arn" {
  type    = string
  default = ""
}

variable "sqs_admission_queue_url" {
  type    = string
  default = ""
}

variable "queue_page_bucket_arn" {
  type    = string
  default = "*"
}

variable "queue_page_url" {
  type    = string
  default = ""
}

variable "queue_join_url" {
  type    = string
  default = ""
}

variable "event_id" {
  type    = string
  default = "evt-001"
}

variable "queue_page_bucket_name" {
  type    = string
  default = ""
}

variable "cors_allowed_origins" {
  type    = string
  default = ""
}
