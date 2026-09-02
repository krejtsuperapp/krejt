terraform {
  backend "s3" {
    bucket       = "krejt-tfstate-prod-7k2m9q"
    key          = "prod/terraform.tfstate"
    region       = "eu-central-1"
    profile      = "krejt-prod"
    encrypt      = true
    use_lockfile = true
  }
}
