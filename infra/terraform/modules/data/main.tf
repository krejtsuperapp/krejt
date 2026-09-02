# -----------------------------------------------------------------------------
# KREJT — të dhënat: Aurora PostgreSQL (§40) + ElastiCache Redis cluster mode (§42).
# Aurora: Serverless v2 në dev (ACU të ulëta), provisioned/më shumë ACU në prod — i njëjti modul.
# Redis: cluster mode ON, Multi-AZ, failover automatik, TLS + auth token në Secrets Manager.
# Të dyja vetëm në subnetet "data" (pa internet), të arritshme vetëm nga SG-ja e aplikacionit.
# -----------------------------------------------------------------------------

# ---------------------------- Aurora PostgreSQL ------------------------------
resource "aws_db_subnet_group" "this" {
  name       = "${var.name}-aurora"
  subnet_ids = var.data_subnet_ids
  tags       = var.tags
}

resource "aws_security_group" "db" {
  name        = "${var.name}-aurora-sg"
  description = "Aurora - ingress only from app tasks"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-aurora-sg" })
}

resource "aws_vpc_security_group_ingress_rule" "db_from_app" {
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = var.app_security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

resource "aws_rds_cluster_parameter_group" "this" {
  name   = "${var.name}-aurora-pg16"
  family = "aurora-postgresql16"
  tags   = var.tags

  parameter {
    name  = "log_min_duration_statement"
    value = "500"
  }
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_rds_cluster" "this" {
  cluster_identifier              = "${var.name}-aurora"
  engine                          = "aurora-postgresql"
  engine_mode                     = "provisioned"
  engine_version                  = var.pg_engine_version
  database_name                   = "krejt"
  master_username                 = "krejt_admin"
  manage_master_user_password     = true
  master_user_secret_kms_key_id   = var.kms_key_arn
  db_subnet_group_name            = aws_db_subnet_group.this.name
  vpc_security_group_ids          = [aws_security_group.db.id]
  db_cluster_parameter_group_name = aws_rds_cluster_parameter_group.this.name
  storage_encrypted               = true
  kms_key_id                      = var.kms_key_arn
  backup_retention_period         = var.backup_retention_days
  preferred_backup_window         = "02:00-03:00"
  preferred_maintenance_window    = "sun:03:30-sun:04:30"
  copy_tags_to_snapshot           = true
  deletion_protection             = var.deletion_protection
  skip_final_snapshot             = !var.deletion_protection
  final_snapshot_identifier       = var.deletion_protection ? "${var.name}-aurora-final" : null
  enabled_cloudwatch_logs_exports = ["postgresql"]
  apply_immediately               = true
  tags                            = var.tags

  serverlessv2_scaling_configuration {
    min_capacity             = var.aurora_min_acu
    max_capacity             = var.aurora_max_acu
    seconds_until_auto_pause = var.aurora_min_acu == 0 ? var.aurora_auto_pause_seconds : null # auto-pause vetëm në dev
  }
}

resource "aws_rds_cluster_instance" "this" {
  count                        = var.aurora_instance_count
  identifier                   = "${var.name}-aurora-${count.index}"
  cluster_identifier           = aws_rds_cluster.this.id
  instance_class               = "db.serverless"
  engine                       = aws_rds_cluster.this.engine
  engine_version               = aws_rds_cluster.this.engine_version
  publicly_accessible          = false
  performance_insights_enabled = true
  tags                         = var.tags
}

# ------------------------------ ElastiCache Redis ----------------------------
resource "aws_elasticache_subnet_group" "this" {
  name       = "${var.name}-redis"
  subnet_ids = var.data_subnet_ids
  tags       = var.tags
}

resource "aws_security_group" "redis" {
  name        = "${var.name}-redis-sg"
  description = "Redis - ingress only from app tasks"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-redis-sg" })
}

resource "aws_vpc_security_group_ingress_rule" "redis_from_app" {
  security_group_id            = aws_security_group.redis.id
  referenced_security_group_id = var.app_security_group_id
  from_port                    = 6379
  to_port                      = 6379
  ip_protocol                  = "tcp"
}

resource "random_password" "redis_auth" {
  length  = 48
  special = false
}

resource "aws_secretsmanager_secret" "redis_auth" {
  name                    = "${var.name}/redis/auth"
  description             = "Auth token për ElastiCache Redis"
  kms_key_id              = var.kms_key_arn
  recovery_window_in_days = 7
  tags                    = var.tags
}

resource "aws_secretsmanager_secret_version" "redis_auth" {
  secret_id     = aws_secretsmanager_secret.redis_auth.id
  secret_string = random_password.redis_auth.result
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id       = "${var.name}-redis"
  description                = "KREJT ${var.name} - GEO, locks, cache, rate limit, realtime state"
  engine                     = "redis"
  engine_version             = var.redis_engine_version
  node_type                  = var.redis_node_type
  port                       = 6379
  parameter_group_name       = "default.redis7.cluster.on"
  num_node_groups            = var.redis_shards
  replicas_per_node_group    = var.redis_replicas_per_shard
  automatic_failover_enabled = true                             # cluster mode e kërkon gjithmonë; pa replika (dev) failover thjesht s'ndodh
  multi_az_enabled           = var.redis_replicas_per_shard > 0 # dev: 1 nyje pa replikë; prod: gjithmonë me replika
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = random_password.redis_auth.result
  kms_key_id                 = var.kms_key_arn
  subnet_group_name          = aws_elasticache_subnet_group.this.name
  security_group_ids         = [aws_security_group.redis.id]
  snapshot_retention_limit   = 1
  snapshot_window            = "03:00-04:00"
  maintenance_window         = "sun:04:30-sun:05:30"
  apply_immediately          = true
  tags                       = var.tags
}
