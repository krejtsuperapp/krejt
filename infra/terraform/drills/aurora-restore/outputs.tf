output "cluster_arn" { value = aws_rds_cluster.drill.arn }
output "secret_arn" { value = one(aws_rds_cluster.drill.master_user_secret[*].secret_arn) }
output "endpoint" { value = aws_rds_cluster.drill.endpoint }
output "restored_to" { value = aws_rds_cluster.drill.restore_to_point_in_time[0].use_latest_restorable_time ? "latest restorable time" : "explicit" }
