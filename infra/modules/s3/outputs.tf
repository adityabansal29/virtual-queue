output "queue_page_bucket_id" { value = aws_s3_bucket.queue_page.id }
output "queue_page_bucket_arn" { value = aws_s3_bucket.queue_page.arn }
output "queue_page_bucket_regional_domain_name" { value = aws_s3_bucket.queue_page.bucket_regional_domain_name }
output "queue_page_oac_id" { value = aws_cloudfront_origin_access_control.queue_page_oac.id }
