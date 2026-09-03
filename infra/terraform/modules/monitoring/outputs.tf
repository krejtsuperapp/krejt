output "alerts_topic_arn" {
  description = "Kanali i alarmeve; modulet e tjera (p.sh. DLQ-ja) publikojnë këtu."
  value       = aws_sns_topic.alerts.arn
}

output "cloudtrail_bucket" {
  value = var.cloudtrail_enabled ? aws_s3_bucket.trail[0].id : null
}
