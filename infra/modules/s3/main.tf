resource "aws_s3_bucket" "queue_page" {
  bucket        = "${var.environment}-virtual-queue-content"
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "queue_page" {
  bucket                  = aws_s3_bucket.queue_page.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "queue_page_oac" {
  name                              = "${var.environment}-queue-page-oac"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}
