output "cluster_name" { value = aws_ecs_cluster.this.name }
output "alb_dns_name" { value = var.alb_enabled ? aws_lb.this[0].dns_name : null }
output "alb_zone_id" { value = var.alb_enabled ? aws_lb.this[0].zone_id : null }
output "app_security_group_id" { value = aws_security_group.app.id }
output "ecr_repository_urls" { value = { for k, r in aws_ecr_repository.this : k => r.repository_url } }
output "task_role_arn" { value = aws_iam_role.task.arn }
output "exec_role_arn" { value = aws_iam_role.exec.arn }
output "ecr_repository_arns" { value = [for k, r in aws_ecr_repository.this : r.arn] }

# Për alarmet: dimensionet e CloudWatch-it kërkojnë prapashtesat e ARN-ve, jo emrat.
output "alb_arn_suffix" { value = local.alb_n == 0 ? null : aws_lb.this[0].arn_suffix }
output "api_target_group_arn_suffix" { value = local.alb_n == 0 ? null : aws_lb_target_group.api[0].arn_suffix }
output "api_service_name" { value = local.alb_n == 0 ? null : aws_ecs_service.api[0].name }
output "worker_service_name" { value = aws_ecs_service.worker.name }
output "centrifugo_service_name" { value = local.alb_n == 0 ? null : aws_ecs_service.centrifugo[0].name }
