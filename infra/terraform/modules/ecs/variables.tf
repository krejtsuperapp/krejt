variable "name" { type = string }
variable "environment" { type = string }
variable "vpc_id" { type = string }
variable "public_subnet_ids" { type = list(string) }
variable "private_subnet_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "assets_bucket_name" { type = string }
variable "assets_bucket_arn" { type = string }
variable "queue_urls" { type = map(string) }
variable "queue_arns" { type = list(string) }
variable "domain_events_topic_arn" { type = string }
variable "secret_arns" {
  description = "Të gjitha sekretet që task-et mund t'i lexojnë (ofruesit + aurora + redis)."
  type        = list(string)
}
variable "aurora_writer_endpoint" { type = string }
variable "aurora_reader_endpoint" { type = string }
variable "aurora_master_secret_arn" { type = string }
variable "redis_endpoint" { type = string }
variable "redis_auth_secret_arn" { type = string }
variable "centrifugo_secret_arn" { type = string }

variable "alb_enabled" {
  description = "Krijo ALB + shërbimet me ALB. false në dev derisa backend-i të ketë imazh."
  type        = bool
  default     = true
}
variable "acm_certificate_arn" {
  description = "Certifikata për HTTPS:443. null derisa domain-i të jetë gati."
  type        = string
  default     = null
}
variable "protect" {
  description = "Mbrojtje nga fshirja (ALB, ECR). true në prod."
  type        = bool
  default     = false
}
variable "image_tag" {
  type    = string
  default = "bootstrap"
}
variable "centrifugo_version" {
  type    = string
  default = "v5"
}
variable "log_retention_days" {
  type    = number
  default = 30
}

variable "api_cpu" {
  type    = number
  default = 512
}
variable "api_memory" {
  type    = number
  default = 1024
}
variable "worker_cpu" {
  type    = number
  default = 512
}
variable "worker_memory" {
  type    = number
  default = 1024
}
variable "api_desired_count" {
  type    = number
  default = 0
}
variable "api_min_count" {
  type    = number
  default = 0
}
variable "api_max_count" {
  type    = number
  default = 6
}
variable "worker_desired_count" {
  type    = number
  default = 0
}
variable "centrifugo_desired_count" {
  type    = number
  default = 0
}
variable "tags" {
  type    = map(string)
  default = {}
}

variable "app_secret_arns" {
  description = "Sekretet e ofruesve sipas emrit (nga moduli security): jwt, google-maps, mapbox-token, infobip, …"
  type        = map(string)
}

variable "public_base_url" {
  description = "Adresa publike e API-së (p.sh. https://staging.krejt.app). Përdoret për lidhjet që ndërton vetë serveri."
  type        = string
}

variable "maps_provider" {
  description = "Ofruesi i hartave: google | mapbox. Të dy sekretet janë të lidhur; kjo vendos cili përdoret."
  type        = string
  default     = "google"

  validation {
    condition     = contains(["google", "mapbox"], var.maps_provider)
    error_message = "maps_provider duhet google ose mapbox."
  }
}

variable "infobip_base_url" {
  description = "Base URL personale e llogarisë Infobip (p.sh. https://xxxxx.api.infobip.com). Adresa e përgjithshme nuk dërgon."
  type        = string
}

variable "infobip_sender" {
  description = "Sender ID i aprovuar për SMS."
  type        = string
  default     = "KREJT"
}

variable "otlp_endpoint" {
  description = "Endpoint-i OTLP i Grafana Cloud (bosh = pa eksport gjurmësh/metrikash)."
  type        = string
  default     = ""
}
