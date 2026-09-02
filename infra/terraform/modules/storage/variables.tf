variable "bucket_name" { type = string }
variable "kms_key_arn" { type = string }
variable "cors_origins" {
  type    = list(string)
  default = ["https://krejt.app", "https://*.krejt.app"]
}
variable "tags" {
  type    = map(string)
  default = {}
}
