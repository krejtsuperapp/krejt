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

# Imazhet publike (logo, produkte, profile) — bucket i veçantë + CloudFront. Në dev lejohen edhe
# panelet lokale si origjina të ngarkimit.
module "media" {
  source       = "../../modules/media"
  bucket_name  = var.media_bucket_name
  cors_origins = ["https://krejt.app", "https://*.krejt.app", "http://localhost:3200", "http://localhost:3300"]
  # Llogaria e re AWS e refuzon CloudFront-in derisa AWS Support ta verifikojë (rast i hapur
  # 04.09.2026). Deri atëherë imazhet i shërben API-ja; kthehet në true pas verifikimit.
  cloudfront_enabled = var.media_cloudfront_enabled
  tags               = local.tags
}

locals {
  media_base_url = module.media.base_url != "" ? module.media.base_url : "${var.public_base_url}/api/v1/media/objects"
}

module "messaging" {
  source          = "../../modules/messaging"
  name            = var.name
  kms_key_id      = module.security.kms_key_id
  alarm_topic_arn = module.monitoring.alerts_topic_arn
  tags            = local.tags
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
  media_bucket_name        = module.media.bucket_name
  media_bucket_arn         = module.media.bucket_arn
  media_base_url           = local.media_base_url
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
  alb_enabled              = var.alb_enabled
  acm_certificate_arn      = local.certificate_arn
  protect                  = false
  api_desired_count        = var.api_desired_count
  api_min_count            = var.api_min_count
  api_max_count            = var.api_max_count
  worker_desired_count     = var.worker_desired_count
  centrifugo_desired_count = var.centrifugo_desired_count
  panel_hosts              = var.panel_hosts
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
  github_repo_id      = var.github_repo_id
  deploy_environment  = "dev"
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

# --- certifikata e paneleve (admin.*, partner.*) ---------------------------------------
# Certifikatë e dytë, e ndarë nga ajo e API-së: kështu API-ja nuk ndalet asnjë sekondë ndërsa
# panelet presin validimin. Lidhet me listener-in vetëm kur është ISSUED (panel_cert_ready).
# Hapat: apply → `terraform output panel_acm_validation_records` → CNAME-t te Cloudflare me
# proxy të fikur → statusi ISSUED → `panel_cert_ready = true` → apply → CNAME-t admin/partner
# → ALB me proxy të ndezur.
resource "aws_acm_certificate" "panels" {
  count                     = length(var.panel_hosts) > 0 ? 1 : 0
  domain_name               = values(var.panel_hosts)[0]
  subject_alternative_names = slice(values(var.panel_hosts), 1, length(var.panel_hosts))
  validation_method         = "DNS"
  tags                      = local.tags

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_listener_certificate" "panels" {
  count           = var.panel_cert_ready && length(var.panel_hosts) > 0 ? 1 : 0
  listener_arn    = module.ecs.https_listener_arn
  certificate_arn = aws_acm_certificate.panels[0].arn
}

# --- monitorimi dhe siguria (§57, §71): alarme te email-i, CloudTrail, GuardDuty ----------
module "monitoring" {
  source                      = "../../modules/monitoring"
  name                        = var.name
  region                      = var.region
  alert_email                 = var.alert_email
  alb_arn_suffix              = module.ecs.alb_arn_suffix
  api_target_group_arn_suffix = module.ecs.api_target_group_arn_suffix
  cluster_name                = module.ecs.cluster_name
  api_service_name            = module.ecs.api_service_name
  worker_service_name         = module.ecs.worker_service_name
  centrifugo_service_name     = module.ecs.centrifugo_service_name
  aurora_cluster_id           = "${var.name}-aurora"
  guardduty_enabled           = var.guardduty_enabled
  tags                        = local.tags
}
