variable "name" { type = string }
variable "region" { type = string }
variable "secret_names" {
  description = "Guaskat e sekreteve të ofruesve (pa vlera)."
  type        = list(string)
  default     = ["google-maps", "fcm", "infobip", "postmark", "payment-provider", "centrifugo", "jwt", "sentry", "posthog", "otel"]
}
variable "tags" {
  type    = map(string)
  default = {}
}
