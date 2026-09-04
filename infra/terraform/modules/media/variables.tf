variable "bucket_name" { type = string }

variable "cloudfront_enabled" {
  description = "CloudFront përpara bucket-it. Një llogari e re AWS e refuzon derisa të verifikohet nga AWS Support; deri atëherë imazhet i shërben API-ja (/api/v1/media/objects)."
  type        = bool
  default     = true
}

variable "cors_origins" {
  description = "Origjinat që ngarkojnë me PUT nga shfletuesi (panelet). Aplikacionet mobile nuk dërgojnë Origin."
  type        = list(string)
  default     = ["https://krejt.app", "https://*.krejt.app"]
}

variable "aliases" {
  description = "Domene të vetat për CloudFront (p.sh. media.krejt.app). Bosh = domeni i CloudFront-it."
  type        = list(string)
  default     = []
}

variable "acm_certificate_arn_us_east_1" {
  description = "Certifikata për aliases; CloudFront-i e kërkon në us-east-1."
  type        = string
  default     = null
}

variable "tags" {
  type    = map(string)
  default = {}
}
