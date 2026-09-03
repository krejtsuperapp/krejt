# =============================================================================
# KREJT — mjedisi STAGING (eu-central-1). Të njëjtat module si dev-i, por me
# konfigurim të plotë: NAT, ALB me certifikatë, Aurora me dy instanca dhe Redis
# me replikë. Staging-u është vendi ku maten 30 ditët pa incident P1 para nisjes
# (§79), ndaj sillet si prodhimi, jo si dev-i.
#
# Ndryshimet ndaj prodhimit janë të qëllimta dhe të pakta:
#   • buxhet më i vogël dhe mbrojtja nga fshirja e fikur, që mjedisi të rindërtohet lirshëm
#   • të dhënat janë prova, ndaj ruajtja e backup-eve është më e shkurtër
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
  cidr        = "10.30.0.0/16"
  az_count    = 3
  nat_enabled = true
  # Një NAT i vetëm: staging-u nuk mban trafik real dhe tre NAT-e kushtojnë sa vetë mjedisi.
  single_nat = true
  tags       = local.tags
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
  bootstrap_admin_phone    = var.bootstrap_admin_phone
  documents_required       = var.documents_required
  dev_test_phones          = var.dev_test_phones
  dev_test_otp             = var.dev_test_otp
  dev_test_admin_phones    = var.dev_test_admin_phones
  sms_provider             = var.sms_provider
  payment_provider         = var.payment_provider
  push_provider            = var.push_provider
  analytics_provider       = var.analytics_provider
  infobip_base_url         = var.infobip_base_url
  infobip_sender           = var.infobip_sender
  otlp_endpoint            = var.otlp_endpoint
  alb_enabled              = true
  acm_certificate_arn      = local.certificate_arn
  protect                  = false
  api_desired_count        = var.api_desired_count
  worker_desired_count     = var.worker_desired_count
  centrifugo_desired_count = var.centrifugo_desired_count
  tags                     = local.tags
}

module "data" {
  source                = "../../modules/data"
  name                  = var.name
  vpc_id                = module.network.vpc_id
  data_subnet_ids       = module.network.data_subnet_ids
  app_security_group_id = module.ecs.app_security_group_id
  kms_key_arn           = module.security.kms_key_arn

  # Dy instanca dhe pa auto-pause: matja e vonesave nuk ka kuptim kur baza fillon nga e ftohta.
  aurora_instance_count     = 2
  aurora_min_acu            = 0.5
  aurora_max_acu            = 8
  aurora_auto_pause_seconds = 0
  backup_retention_days     = 7
  deletion_protection       = false

  redis_node_type          = "cache.t4g.small"
  redis_shards             = 1
  redis_replicas_per_shard = 1
  tags                     = local.tags
}

resource "aws_budgets_budget" "staging" {
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

module "cicd" {
  source              = "../../modules/cicd"
  name                = var.name
  region              = var.region
  github_repo         = var.github_repo
  github_repo_id      = var.github_repo_id
  deploy_environment  = "staging"
  cluster_name        = module.ecs.cluster_name
  ecr_repository_arns = module.ecs.ecr_repository_arns
  task_role_arn       = module.ecs.task_role_arn
  exec_role_arn       = module.ecs.exec_role_arn
  tags                = local.tags
}

# --- certifikata e ALB-së ------------------------------------------------------
# E mban Terraform-i, që të mos mbetet burim i krijuar me dorë. Validimi bëhet me DNS:
# pas apply-t të parë, `terraform output acm_validation_record` jep regjistrimin që
# duhet shtuar te Cloudflare **me proxy të fikur** — validimi i ACM-së nuk kalon përmes tij.
resource "aws_acm_certificate" "api" {
  count             = var.domain_name == "" ? 0 : 1
  domain_name       = var.domain_name
  validation_method = "DNS"
  tags              = local.tags

  lifecycle {
    create_before_destroy = true
  }
}

locals {
  # Certifikata e vetë Terraform-it ka përparësi; `acm_certificate_arn` mbetet për një
  # certifikatë të lëshuar diku tjetër.
  certificate_arn = var.domain_name == "" ? var.acm_certificate_arn : one(aws_acm_certificate.api[*].arn)
}
