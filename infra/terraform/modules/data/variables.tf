variable "name" { type = string }
variable "vpc_id" { type = string }
variable "data_subnet_ids" { type = list(string) }
variable "app_security_group_id" { type = string }
variable "kms_key_arn" { type = string }

# Aurora
variable "pg_engine_version" {
  type    = string
  default = "16.14" # versioni më i ri 16.x në eu-central-1 (02.09.2026); familja e parametrave aurora-postgresql16
}
variable "aurora_instance_count" {
  description = "1 në dev (writer), 2+ në prod (writer + reader)."
  type        = number
  default     = 1
}
variable "aurora_min_acu" {
  type    = number
  default = 0.5
}
variable "aurora_max_acu" {
  type    = number
  default = 4
}
variable "aurora_auto_pause_seconds" {
  description = "Pas sa sekondash pa aktivitet Aurora shkon në 0 ACU (vetëm kur min_acu = 0)."
  type        = number
  default     = 600
}
variable "backup_retention_days" {
  type    = number
  default = 7
}
variable "deletion_protection" {
  type    = bool
  default = false
}

# Redis
variable "redis_engine_version" {
  type    = string
  default = "7.1"
}
variable "redis_node_type" {
  type    = string
  default = "cache.t4g.small"
}
variable "redis_shards" {
  type    = number
  default = 1
}
variable "redis_replicas_per_shard" {
  type    = number
  default = 1
}

variable "tags" {
  type    = map(string)
  default = {}
}
