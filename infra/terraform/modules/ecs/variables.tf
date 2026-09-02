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
  description = "Sekretet e ofruesve sipas emrit (nga moduli security): jwt, google-maps, infobip, …"
  type        = map(string)
}

variable "otlp_endpoint" {
  description = "Endpoint-i OTLP i Grafana Cloud (bosh = pa eksport gjurmësh/metrikash)."
  type        = string
  default     = ""
}
