terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

module "networking" {
  source      = "../../modules/networking"
  environment = var.environment
  vpc_cidr    = var.vpc_cidr
}

module "ecs" {
  source                       = "../../modules/ecs"
  environment                  = var.environment
  aws_region                   = var.aws_region
  vpc_id                       = module.networking.vpc_id
  private_subnet_ids           = module.networking.private_subnet_ids
  public_subnet_ids            = module.networking.public_subnet_ids
  sg_ecs_tasks_id              = module.networking.sg_ecs_tasks_id
  sg_alb_public_id             = module.networking.sg_alb_public_id
  sg_alb_stub_origin_id        = module.networking.sg_alb_stub_origin_id
  redis_queue_addr             = module.redis.redis_queue_primary_endpoint
  redis_origin_addr            = module.redis.redis_origin_primary_endpoint
  ssm_admission_secret_arn     = data.aws_ssm_parameter.admission_secret.arn
  ssm_session_secret_arn       = data.aws_ssm_parameter.session_secret.arn
  dynamodb_sessions_table_arn  = module.dynamodb.queue_sessions_table_arn
  dynamodb_events_table_arn    = module.dynamodb.queue_events_table_arn
  dynamodb_audit_log_table_arn = module.dynamodb.queue_audit_log_table_arn
  sqs_admission_queue_arn      = module.sqs.admission_events_queue_arn
  sqs_admission_queue_url      = module.sqs.admission_events_queue_url
  queue_page_bucket_arn        = module.s3.queue_page_bucket_arn
  queue_page_url               = "https://${module.cloudfront.queue_page_cf_domain}/queue/"
  queue_page_bucket_name       = module.s3.queue_page_bucket_id
  cors_allowed_origins         = "https://${module.cloudfront.queue_page_cf_domain}"
}

module "redis" {
  source             = "../../modules/redis"
  environment        = var.environment
  redis_node_type    = var.redis_node_type
  vpc_id             = module.networking.vpc_id
  private_subnet_ids = module.networking.private_subnet_ids
  sg_redis_id        = module.networking.sg_redis_id
}

module "dynamodb" {
  source      = "../../modules/dynamodb"
  environment = var.environment
}

module "sqs" {
  source      = "../../modules/sqs"
  environment = var.environment
}

# PREREQUISITE: Create SSM SecureString parameters before first apply:
# aws ssm put-parameter --name "/virtual-queue/dev/ADMISSION_SECRET" \
#   --type SecureString --value "<secret>" --region ap-south-1
# aws ssm put-parameter --name "/virtual-queue/dev/SESSION_SECRET" \
#   --type SecureString --value "<secret>" --region ap-south-1
data "aws_ssm_parameter" "admission_secret" {
  name = "/virtual-queue/dev/ADMISSION_SECRET"
}

data "aws_ssm_parameter" "session_secret" {
  name = "/virtual-queue/dev/SESSION_SECRET"
}

module "s3" {
  source      = "../../modules/s3"
  environment = var.environment
}

module "cloudfront" {
  source                                 = "../../modules/cloudfront"
  environment                            = var.environment
  queue_page_bucket_id                   = module.s3.queue_page_bucket_id
  queue_page_bucket_arn                  = module.s3.queue_page_bucket_arn
  queue_page_bucket_regional_domain_name = module.s3.queue_page_bucket_regional_domain_name
  queue_page_oac_id                      = module.s3.queue_page_oac_id
}

module "cloudfront_api" {
  source      = "../../modules/cloudfront-api"
  environment = var.environment
  alb_dns     = module.ecs.alb_queue_api_dns
  depends_on  = [module.ecs]
}

resource "aws_s3_object" "queue_index" {
  bucket       = module.s3.queue_page_bucket_id
  key          = "queue/index.html"
  content      = replace(file("${path.root}/../../../web/queue/index.html"), "__QUEUE_API_BASE__", "https://${module.cloudfront_api.api_cf_domain}")
  content_type = "text/html"
}
