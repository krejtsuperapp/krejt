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

variable "maps_provider" {
  description = "google | mapbox"
  type        = string
  default     = "google"
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

variable "github_repo" {
  description = "Repo-ja që bën deploy përmes OIDC (owner/repo)."
  type        = string
  default     = "krejtsuperapp/krejt"
}
