variable "region" {
  type    = string
  default = "eu-central-1"
}
variable "aws_profile" {
  type    = string
  default = "krejt-dev"
}
variable "environment" {
  type    = string
  default = "dev"
}
variable "name" {
  description = "Prefiksi i të gjitha burimeve."
  type        = string
  default     = "krejt-dev"
}
variable "assets_bucket_name" {
  type = string
}
variable "domain_name" {
  description = "Domeni i API-së për këtë mjedis. Bosh = certifikata nuk krijohet nga Terraform."
  type        = string
  default     = "dev.krejt.app"
}

variable "acm_certificate_arn" {
  type    = string
  default = null
}
variable "nat_enabled" {
  type    = bool
  default = false
}
variable "alb_enabled" {
  type    = bool
  default = false
}
# Numri i detyrave nis nga zero: mjedisi ekziston pa shpenzuar për detyra që do të
# dështonin derisa imazhi i parë të jetë në ECR.
variable "api_min_count" {
  description = "Kufiri i poshtëm i autoscaling-ut. Zero do të thotë që API-ja mund të ulet vetvetiu deri në asnjë detyrë."
  type        = number
  default     = 1
}

variable "api_max_count" {
  type    = number
  default = 6
}

variable "api_desired_count" {
  type    = number
  default = 0
}
variable "worker_desired_count" {
  type    = number
  default = 0
}
variable "centrifugo_desired_count" {
  type    = number
  default = 0
}

variable "monthly_budget_usd" {
  type    = number
  default = 60
}
variable "alert_email" {
  type = string
}

variable "public_base_url" {
  description = "Adresa publike e API-së për këtë mjedis."
  type        = string
}

variable "bootstrap_admin_phone" {
  description = "Numri (E.164) që merr SUPER_ADMIN në nisje, vetëm nëse ende nuk ka asnjë administrator."
  type        = string
  default     = ""
}

variable "documents_required" {
  description = "Dokumentet e miratuara para aprovimit të shoferit. false lejohet vetëm në development."
  type        = bool
  default     = true
}

variable "sms_provider" {
  description = "infobip | devlog (vetëm development)"
  type        = string
  default     = "infobip"
}

variable "payment_provider" {
  description = "stripe | devlog (vetëm development)"
  type        = string
  default     = "stripe"
}

variable "push_provider" {
  description = "fcm | devlog (vetëm development)"
  type        = string
  default     = "fcm"
}

variable "analytics_provider" {
  description = "posthog | devlog (vetëm development)"
  type        = string
  default     = "posthog"
}

variable "maps_provider" {
  description = "google | mapbox"
  type        = string
  default     = "mapbox"
}

variable "infobip_base_url" {
  description = "Base URL personale e llogarisë Infobip; e gjen te paneli i tyre."
  type        = string
}

variable "infobip_sender" {
  description = "Sender ID i aprovuar."
  type        = string
  default     = "KREJT"
}

variable "otlp_endpoint" {
  description = "Grafana Cloud OTLP gateway (p.sh. https://otlp-gateway-prod-eu-west-2.grafana.net/otlp); bosh = pa eksport."
  type        = string
  default     = ""
}

variable "github_repo_id" {
  description = "Forma e pandryshueshme e repo-s te token-i i OIDC-së (owner@ownerId/repo@repoId)."
  type        = string
  default     = "krejtsuperapp@323840566/krejt@1354309985"
}

variable "github_repo" {
  description = "Repo-ja që bën deploy përmes OIDC (owner/repo)."
  type        = string
  default     = "krejtsuperapp/krejt"
}
