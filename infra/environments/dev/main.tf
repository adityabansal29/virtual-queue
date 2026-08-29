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

# implemented in 03-02
module "ecs" {
  source      = "../../modules/ecs"
  environment = var.environment
}

# implemented in 03-02
module "redis" {
  source = "../../modules/redis"
}

# implemented in 03-02
module "dynamodb" {
  source = "../../modules/dynamodb"
}

# implemented in 03-02
module "sqs" {
  source = "../../modules/sqs"
}

# implemented in 03-04/03-05
module "s3" {
  source = "../../modules/s3"
}

# implemented in 03-04/03-05
module "cloudfront" {
  source = "../../modules/cloudfront"
}
