resource "aws_ecr_repository" "queueserver" {
  name                 = "${var.environment}-queueserver"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

resource "aws_ecr_repository" "scheduler" {
  name                 = "${var.environment}-scheduler"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

resource "aws_ecr_repository" "stuborigin" {
  name                 = "${var.environment}-stuborigin"
  image_tag_mutability = "MUTABLE"
  force_delete         = true
}

locals {
  ecr_lifecycle_policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep the last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  })
}

resource "aws_ecr_lifecycle_policy" "queueserver" {
  repository = aws_ecr_repository.queueserver.name
  policy     = local.ecr_lifecycle_policy
}

resource "aws_ecr_lifecycle_policy" "scheduler" {
  repository = aws_ecr_repository.scheduler.name
  policy     = local.ecr_lifecycle_policy
}

resource "aws_ecr_lifecycle_policy" "stuborigin" {
  repository = aws_ecr_repository.stuborigin.name
  policy     = local.ecr_lifecycle_policy
}
