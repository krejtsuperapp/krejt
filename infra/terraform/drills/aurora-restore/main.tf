# =============================================================================
# KREJT — ushtrim rikthimi i Aurora-s (§71, runbook "Rikthimi").
#
# Ngre një cluster të përkohshëm nga kopja rezervë e cluster-it burim (point-in-time, momenti
# i fundit i rikthyeshëm), me Data API të ndezur që verifikimi të bëhet me një pyetje SQL pa
# hyrë në VPC. Pas verifikimit: `terraform destroy`. Gjendja mbahet lokalisht: cluster-i jeton
# më pak se një orë dhe nuk i takon asnjë mjedisi.
#
#   terraform init && terraform apply -var source_cluster=krejt-dev-aurora
#   aws rds-data execute-statement --resource-arn $(terraform output -raw cluster_arn) \
#     --secret-arn $(terraform output -raw secret_arn) --database krejt \
#     --sql "select count(*) from users"
#   terraform destroy
# =============================================================================

data "aws_rds_cluster" "source" {
  cluster_identifier = var.source_cluster
}

resource "aws_rds_cluster" "drill" {
  cluster_identifier = "${var.source_cluster}-drill"
  engine             = data.aws_rds_cluster.source.engine
  engine_mode        = "provisioned"

  restore_to_point_in_time {
    source_cluster_identifier  = var.source_cluster
    restore_type               = "full-copy"
    use_latest_restorable_time = true
  }

  db_subnet_group_name   = data.aws_rds_cluster.source.db_subnet_group_name
  vpc_security_group_ids = data.aws_rds_cluster.source.vpc_security_group_ids
  kms_key_id             = data.aws_rds_cluster.source.kms_key_id

  # Fjalëkalimi i rikthyer është ai i burimit në momentin e kopjes; nuk na duhet: RDS-i
  # gjeneron një sekret të ri vetëm për këtë cluster, që Data API ta përdorë.
  manage_master_user_password   = true
  master_user_secret_kms_key_id = data.aws_rds_cluster.source.kms_key_id
  enable_http_endpoint          = true

  serverlessv2_scaling_configuration {
    min_capacity = 0.5
    max_capacity = 1
  }

  backup_retention_period = 1
  deletion_protection     = false
  skip_final_snapshot     = true
  apply_immediately       = true

  tags = { Project = "krejt", Purpose = "backup-drill", ManagedBy = "terraform" }
}

resource "aws_rds_cluster_instance" "drill" {
  identifier         = "${var.source_cluster}-drill-1"
  cluster_identifier = aws_rds_cluster.drill.id
  instance_class     = "db.serverless"
  engine             = aws_rds_cluster.drill.engine
  engine_version     = aws_rds_cluster.drill.engine_version
  tags               = { Project = "krejt", Purpose = "backup-drill", ManagedBy = "terraform" }
}
