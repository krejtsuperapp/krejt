variable "name" { type = string }
variable "kms_key_id" { type = string }
variable "alarm_topic_arn" {
  type    = string
  default = null
}
variable "tags" {
  type    = map(string)
  default = {}
}
