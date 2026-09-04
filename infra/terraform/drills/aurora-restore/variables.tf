variable "region" {
  type    = string
  default = "eu-central-1"
}

variable "aws_profile" {
  type    = string
  default = "krejt-dev"
}

variable "source_cluster" {
  description = "Cluster-i Aurora nga i cili rikthehet kopja (p.sh. krejt-dev-aurora)."
  type        = string
}
