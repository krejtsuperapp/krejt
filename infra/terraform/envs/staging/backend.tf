terraform {
  backend "s3" {
    bucket       = "krejt-tfstate-staging-7k2m9q"
    key          = "staging/terraform.tfstate"
    region       = "eu-central-1"
    profile      = "krejt-staging"
    encrypt      = true
    use_lockfile = true
  }
}
