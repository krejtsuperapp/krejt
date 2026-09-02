# =============================================================================
# KREJT — mjedisi DEV (eu-central-1) — "dev i lehtë" (vendim 02.09.2026):
#   • Aurora Serverless v2 me auto-pause (0 ACU kur s'përdoret)
#   • Redis 1 nyje cache.t4g.micro, pa Multi-AZ
#   • pa NAT dhe pa ALB derisa backend-i të ketë imazhin e parë (nat_enabled / alb_enabled → true)
#   • buxhet 60 USD/muaj me alarm
# Staging dhe prod përdorin të njëjtat module me konfigurim të plotë HA.
# Rendi: security → network → storage → messaging → ecs → data.
# =============================================================================
locals {
  tags = { Project = "krejt", Environment = var.environment, ManagedBy = "terraform" }
}

module "security" {
  source = "../../modules/security"
  name   = var.name
  region = var.region
  tags   = local.tags
}

module "network" {
  source      = "../../modules/network"
  name        = var.name
  region      = var.region
  cidr        = "10.20.0.0/16"
  az_count    = 3
  nat_enabled = var.nat_enabled
  single_nat  = true
  tags        = local.tags
}

module "storage" {
  source      = "../../modules/storage"
  bucket_name = var.assets_bucket_name
  kms_key_arn = module.security.kms_key_arn
  tags        = local.tags
}

module "messaging" {
  source     = "../../modules/messaging"
  name       = var.name
  kms_key_id = module.security.kms_key_id
  tags       = local.tags
}

module "ecs" {
  source                   = "../../modules/ecs"
  name                     = var.name
  environment              = var.environment
  vpc_id                   = module.network.vpc_id
  public_subnet_ids        = module.network.public_subnet_ids
  private_subnet_ids       = module.network.private_subnet_ids
  kms_key_arn              = module.security.kms_key_arn
  assets_bucket_name       = module.storage.bucket_name
  assets_bucket_arn        = module.storage.bucket_arn
  queue_urls               = module.messaging.queue_urls
  queue_arns               = values(module.messaging.queue_arns)
  domain_events_topic_arn  = module.messaging.domain_events_topic_arn
  secret_arns              = concat(values(module.security.secret_arns), [module.data.aurora_master_secret_arn, module.data.redis_auth_secret_arn])
  aurora_writer_endpoint   = module.data.aurora_writer_endpoint
  aurora_reader_endpoint   = module.data.aurora_reader_endpoint
  aurora_master_secret_arn = module.data.aurora_master_secret_arn
  redis_endpoint           = module.data.redis_configuration_endpoint
  redis_auth_secret_arn    = module.data.redis_auth_secret_arn
  centrifugo_secret_arn    = module.security.secret_arns["centrifugo"]
  app_secret_arns          = module.security.secret_arns
  public_base_url          = var.public_base_url
  maps_provider            = var.maps_provider
  sms_provider             = var.sms_provider
  payment_provider         = var.payment_provider
  push_provider            = var.push_provider
  analytics_provider       = var.analytics_provider
  infobip_base_url         = var.infobip_base_url
  infobip_sender           = var.infobip_sender
  otlp_endpoint            = var.otlp_endpoint
  alb_enabled              = var.alb_enabled
  acm_certificate_arn      = var.acm_certificate_arn
  protect                  = false
  api_desired_count        = var.api_desired_count
  worker_desired_count     = var.worker_desired_count
  centrifugo_desired_count = var.centrifugo_desired_count
  tags                     = local.tags
}

module "data" {
  source                    = "../../modules/data"
  name                      = var.name
  vpc_id                    = module.network.vpc_id
  data_subnet_ids           = module.network.data_subnet_ids
  app_security_group_id     = module.ecs.app_security_group_id
  kms_key_arn               = module.security.kms_key_arn
  aurora_instance_count     = 1
  aurora_min_acu            = 0 # auto-pause në dev
  aurora_max_acu            = 4
  aurora_auto_pause_seconds = 600
  backup_retention_days     = 7
  deletion_protection       = false
  redis_node_type           = "cache.t4g.micro"
  redis_shards              = 1
  redis_replicas_per_shard  = 0 # dev: pa replikë; prod: ≥ 1
  tags                      = local.tags
}

# --- buxheti i dev-it: alarm në 80 % dhe 100 % të 60 USD ------------------------
resource "aws_budgets_budget" "dev" {
  name         = "${var.name}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.alert_email]
  }
  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 100
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.alert_email]
  }
}

# --- CI/CD: GitHub Actions → ECR/ECS me OIDC (asnjë çelës i përhershëm) --------------------
module "cicd" {
  source              = "../../modules/cicd"
  name                = var.name
  region              = var.region
  github_repo         = var.github_repo
  cluster_name        = module.ecs.cluster_name
  ecr_repository_arns = module.ecs.ecr_repository_arns
  task_role_arn       = module.ecs.task_role_arn
  exec_role_arn       = module.ecs.exec_role_arn
  tags                = local.tags
}
