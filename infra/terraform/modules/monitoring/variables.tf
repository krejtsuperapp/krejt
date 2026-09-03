variable "name" { type = string }
variable "region" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}

variable "alert_email" {
  description = "Ku shkojnë alarmet. Abonimi kërkon konfirmim një herë nga email-i."
  type        = string
}

# --- dimensionet e alarmeve (null = alarmi nuk krijohet) --------------------------------
variable "alb_arn_suffix" {
  type    = string
  default = null
}
variable "api_target_group_arn_suffix" {
  type    = string
  default = null
}
variable "cluster_name" { type = string }
variable "api_service_name" {
  type    = string
  default = null
}
variable "worker_service_name" {
  type    = string
  default = null
}
variable "centrifugo_service_name" {
  type    = string
  default = null
}
variable "aurora_cluster_id" { type = string }

# --- pragjet ------------------------------------------------------------------------------
variable "latency_p95_seconds" {
  description = "Koha e përgjigjes p95 e ALB-së mbi të cilën bie alarmi."
  type        = number
  default     = 1.5
}
variable "target_5xx_per_5min" {
  description = "Gabime 5xx nga aplikacioni brenda 5 minutash mbi të cilat bie alarmi."
  type        = number
  default     = 10
}

# --- siguria ------------------------------------------------------------------------------
variable "cloudtrail_enabled" {
  description = "Gjurma e çdo thirrjeje API në llogari, në një kovë të mbyllur. Praktikisht falas për një gjurmë."
  type        = bool
  default     = true
}
variable "guardduty_enabled" {
  description = "Zbulim kërcënimesh nga AWS-i (kredenciale të kompromentuara, sjellje anormale). Disa dollarë në muaj në këtë madhësi."
  type        = bool
  default     = true
}
variable "cloudtrail_retention_days" {
  type    = number
  default = 365
}
