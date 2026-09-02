output "cluster_name" { value = aws_ecs_cluster.this.name }
output "alb_dns_name" { value = var.alb_enabled ? aws_lb.this[0].dns_name : null }
output "alb_zone_id" { value = var.alb_enabled ? aws_lb.this[0].zone_id : null }
output "app_security_group_id" { value = aws_security_group.app.id }
output "ecr_repository_urls" { value = { for k, r in aws_ecr_repository.this : k => r.repository_url } }
output "task_role_arn" { value = aws_iam_role.task.arn }
output "exec_role_arn" { value = aws_iam_role.exec.arn }
