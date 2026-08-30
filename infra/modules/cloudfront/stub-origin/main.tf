resource "aws_cloudfront_key_value_store" "queue_secrets" { name = "${var.environment}-queue-secrets" }

resource "aws_cloudfront_function" "stub_origin_guard" {
  name    = "${var.environment}-stub-origin-guard"
  runtime = "cloudfront-js-2.0"
  publish = true
  code = templatefile("${path.module}/functions/stub_origin_guard.js", {
    # cf.kvs() requires the UUID namespace, not the Terraform resource name.
    kvs_id = regex("[^/]+$", aws_cloudfront_key_value_store.queue_secrets.arn)
    queue_join_host = var.queue_join_host
  })
  key_value_store_associations = [aws_cloudfront_key_value_store.queue_secrets.arn]
}

resource "aws_cloudfront_distribution" "stub_origin" {
  enabled = true
  price_class = "PriceClass_100"
  origin {
    domain_name = var.stub_origin_alb_dns
    origin_id = "alb-stub-origin"
    custom_origin_config {
      http_port = 80
      https_port = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols = ["TLSv1.2"]
    }
  }
  default_cache_behavior {
    target_origin_id = "alb-stub-origin"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods = ["GET", "HEAD"]
    cache_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
    origin_request_policy_id = "216adef6-5c7f-47e4-b989-5492eafa07d3"
    function_association {
      event_type = "viewer-request"
      function_arn = aws_cloudfront_function.stub_origin_guard.arn
    }
  }
  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }
  viewer_certificate { cloudfront_default_certificate = true }
}

# POST-APPLY: populate KVS from SSM without storing secret values in Terraform state.
# KVS_ARN=$(terraform output -raw stub_origin_kvs_arn)
# ETAG=$(aws cloudfront-keyvaluestore describe-key-value-store --kvs-arn "$KVS_ARN" --query ETag --output text)
# for KEY in ADMISSION_SECRET SESSION_SECRET; do
#   VALUE=$(aws ssm get-parameter --name "/virtual-queue/dev/$KEY" --with-decryption --query Parameter.Value --output text --region ap-south-1)
#   aws cloudfront-keyvaluestore put-key --kvs-arn "$KVS_ARN" --key "$KEY" --value "$VALUE" --if-match "$ETAG"
#   ETAG=$(aws cloudfront-keyvaluestore describe-key-value-store --kvs-arn "$KVS_ARN" --query ETag --output text)
# done
