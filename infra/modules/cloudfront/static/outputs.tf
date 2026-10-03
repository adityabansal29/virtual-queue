output "queue_page_cf_domain" {
  value = aws_cloudfront_distribution.queue_page.domain_name
}

output "queue_page_distribution_id" {
  value = aws_cloudfront_distribution.queue_page.id
}
