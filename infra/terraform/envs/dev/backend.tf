terraform {
  backend "s3" {
    bucket       = "krejt-tfstate-dev-7k2m9q"
    key          = "dev/terraform.tfstate"
    region       = "eu-central-1"
    profile      = "krejt-dev"
    encrypt      = true
    use_lockfile = true # locking native në S3 (Terraform ≥ 1.10); tabela DynamoDB krejt-tflock mbetet si rezervë
  }
}
