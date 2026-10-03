output "stub_origin_cf_domain" { value = module.cloudfront_stub_origin.stub_origin_cf_domain }
output "stub_origin_kvs_arn" { value = module.cloudfront_stub_origin.kvs_arn }
output "queue_api_cf_domain" { value = module.cloudfront_api.queue_api_cf_domain }
output "queue_page_cf_domain" { value = module.cloudfront.queue_page_cf_domain }
output "queue_page_distribution_id" { value = module.cloudfront.queue_page_distribution_id }
