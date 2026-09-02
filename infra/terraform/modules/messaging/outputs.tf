output "queue_urls" { value = { for k, q in aws_sqs_queue.this : k => q.url } }
output "queue_arns" { value = { for k, q in aws_sqs_queue.this : k => q.arn } }
output "dlq_arns" { value = { for k, q in aws_sqs_queue.dlq : k => q.arn } }
output "domain_events_topic_arn" { value = aws_sns_topic.domain_events.arn }
