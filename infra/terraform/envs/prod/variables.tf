variable "region" {
  type    = string
  default = "eu-central-1"
}
variable "aws_profile" {
  type    = string
  default = "krejt-prod"
}
variable "environment" {
  type    = string
  default = "prod"
}
variable "name" {
  description = "Prefiksi i të gjitha burimeve."
  type        = string
  default     = "krejt-prod"
}
variable "assets_bucket_name" {
  type = string
}

variable "acm_certificate_arn" {
  description = "Certifikata për krejt.app. ALB-ja nuk ngrihet pa të."
  type        = string
}

# Numri i detyrave rritet pasi imazhi i parë të jetë në ECR; deri atëherë mjedisi
# ekziston por nuk shpenzon për detyra që do të dështonin gjithsesi.
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
  default = 600
}
variable "alert_email" {
  type = string
}

variable "otlp_endpoint" {
  description = "Grafana Cloud OTLP gateway; bosh = pa eksport."
  type        = string
  default     = ""
}

variable "github_repo" {
  description = "Repo-ja që bën deploy përmes OIDC (owner/repo)."
  type        = string
  default     = "krejtsuperapp/krejt"
}
