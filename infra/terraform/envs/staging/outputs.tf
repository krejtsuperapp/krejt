output "vpc_id" { value = module.network.vpc_id }
output "alb_dns_name" { value = module.ecs.alb_dns_name }
output "ecr_repository_urls" { value = module.ecs.ecr_repository_urls }
output "aurora_writer_endpoint" { value = module.data.aurora_writer_endpoint }
output "aurora_reader_endpoint" { value = module.data.aurora_reader_endpoint }
output "aurora_master_secret_arn" { value = module.data.aurora_master_secret_arn }
output "redis_configuration_endpoint" { value = module.data.redis_configuration_endpoint }
output "redis_auth_secret_arn" { value = module.data.redis_auth_secret_arn }
output "queue_urls" { value = module.messaging.queue_urls }
output "domain_events_topic_arn" { value = module.messaging.domain_events_topic_arn }
output "assets_bucket" { value = module.storage.bucket_name }
output "kms_key_arn" { value = module.security.kms_key_arn }
output "provider_secret_arns" { value = module.security.secret_arns }
output "deploy_role_arn" { value = module.cicd.deploy_role_arn }
output "ecs_cluster_name" { value = module.ecs.cluster_name }

# Regjistrimi që duhet shtuar te Cloudflare për ta validuar certifikatën.
output "acm_validation_record" {
  description = "CNAME i validimit; shtohet te Cloudflare me proxy të fikur."
  value = try({
    emri  = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_name
    lloji = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_type
    vlera = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_value
  }, null)
}

output "acm_certificate_arn" { value = local.certificate_arn }
