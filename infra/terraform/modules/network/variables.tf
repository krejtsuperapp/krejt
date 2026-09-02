variable "name" { type = string }
variable "region" { type = string }
variable "cidr" {
  type    = string
  default = "10.20.0.0/16"
}
variable "az_count" {
  type    = number
  default = 3
}
variable "nat_enabled" {
  description = "Krijo NAT Gateway. false në dev derisa task-et të kenë nevojë për dalje në internet."
  type        = bool
  default     = true
}
variable "single_nat" {
  description = "Një NAT për gjithë VPC-në (dev). Në prod: false → një NAT për AZ."
  type        = bool
  default     = true
}
variable "flow_log_retention_days" {
  type    = number
  default = 30
}
variable "tags" {
  type    = map(string)
  default = {}
}
