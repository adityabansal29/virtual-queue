resource "aws_cloudfront_distribution" "queue_page" {
  enabled             = true
  default_root_object = "queue/index.html"
  price_class         = "PriceClass_100"

  origin {
    domain_name              = var.queue_page_bucket_regional_domain_name
    origin_id                = "s3-queue-page"
    origin_access_control_id = var.queue_page_oac_id
  }

  default_cache_behavior {
    target_origin_id       = "s3-queue-page"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    cache_policy_id        = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }
  viewer_certificate { cloudfront_default_certificate = true }
}

resource "aws_s3_bucket_policy" "queue_page" {
  bucket = var.queue_page_bucket_id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "cloudfront.amazonaws.com" }
      Action    = "s3:GetObject"
      Resource  = "${var.queue_page_bucket_arn}/*"
      Condition = { StringEquals = { "AWS:SourceArn" = aws_cloudfront_distribution.queue_page.arn } }
    }]
  })
}
