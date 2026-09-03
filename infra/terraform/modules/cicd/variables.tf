variable "name" { type = string }
variable "region" { type = string }
variable "github_repo" {
  description = "owner/repo që lejohet të bëjë deploy (OIDC subject)."
  type        = string
}
variable "deploy_environment" {
  description = "Mjedisi i GitHub-it nga i cili lejohet deploy-i (puna deklaron environment:)."
  type        = string
}

variable "deploy_branch" {
  type    = string
  default = "main"
}
variable "cluster_name" { type = string }
variable "ecr_repository_arns" { type = list(string) }
variable "task_role_arn" { type = string }
variable "exec_role_arn" { type = string }
variable "tags" {
  type    = map(string)
  default = {}
}
