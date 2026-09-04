variable "region" {
  type    = string
  default = "eu-central-1"
}
variable "aws_profile" {
  type    = string
  default = "krejt-staging"
}
variable "environment" {
  type    = string
  default = "staging"
}
variable "name" {
  description = "Prefiksi i të gjitha burimeve."
  type        = string
  default     = "krejt-staging"
}
variable "assets_bucket_name" {
  type = string
}
variable "media_bucket_name" {
  description = "Bucket-i i imazheve publike (lexohet përmes CloudFront-it). Emri është global në S3."
  type        = string
}
variable "media_cloudfront_enabled" {
  description = "CloudFront përpara imazheve. false derisa AWS Support ta verifikojë llogarinë (refuzon krijimin)."
  type        = bool
  default     = false
}

variable "domain_name" {
  description = "Domeni i API-së për këtë mjedis. Bosh = certifikata nuk krijohet nga Terraform."
  type        = string
  default     = "staging.krejt.app"
}

variable "acm_certificate_arn" {
  description = "Certifikata për staging.krejt.app. ALB-ja nuk ngrihet pa të."
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
  default = 200
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

variable "dev_test_phones" {
  description = "Numra prove (E.164) me kod fiks, pa SMS. Vetëm development."
  type        = list(string)
  default     = []
}

variable "dev_test_admin_phones" {
  description = "Nënbashkësia e numrave të provës që marrin SUPER_ADMIN. Vetëm development."
  type        = list(string)
  default     = []
}

variable "dev_test_otp" {
  description = "Kodi fiks i numrave të provës. Vetëm development."
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
  description = "Grafana Cloud OTLP gateway; bosh = pa eksport."
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

variable "guardduty_enabled" {
  description = "Zbulim kërcënimesh nga AWS-i. Disa dollarë në muaj në këtë madhësi; në prodhim mbetet i ndezur."
  type        = bool
  default     = true
}
