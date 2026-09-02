output "aurora_writer_endpoint" { value = aws_rds_cluster.this.endpoint }
output "aurora_reader_endpoint" { value = aws_rds_cluster.this.reader_endpoint }
output "aurora_database" { value = aws_rds_cluster.this.database_name }
output "aurora_master_secret_arn" {
  description = "Secrets Manager — username/password i menaxhuar nga RDS"
  value       = try(aws_rds_cluster.this.master_user_secret[0].secret_arn, null)
}
output "redis_configuration_endpoint" { value = aws_elasticache_replication_group.this.configuration_endpoint_address }
output "redis_auth_secret_arn" { value = aws_secretsmanager_secret.redis_auth.arn }
output "db_security_group_id" { value = aws_security_group.db.id }
output "redis_security_group_id" { value = aws_security_group.redis.id }
