output "queue_page_cf_domain" { value = aws_cloudfront_distribution.queue_page.domain_name }
output "events_cf_domain" { value = aws_cloudfront_distribution.events.domain_name }
