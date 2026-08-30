output "stub_origin_cf_domain" { value = aws_cloudfront_distribution.stub_origin.domain_name }
output "kvs_arn" { value = aws_cloudfront_key_value_store.queue_secrets.arn }
