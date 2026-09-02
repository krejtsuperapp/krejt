# -----------------------------------------------------------------------------
# KREJT — radhët (SQS) dhe ngjarjet (SNS). Çdo radhë ka DLQ. Të gjitha të enkriptuara me KMS.
#   dispatch   FIFO  — ofertat e dispatch-it (rendi për udhëtim/porosi, dedup)
#   payouts    FIFO  — payout-et (asnjëherë dy herë)
#   outbox     std   — publikuesi i transactional outbox → SNS
#   notifications std — push / email / SMS
# -----------------------------------------------------------------------------
locals {
  queues = {
    dispatch      = { fifo = true, visibility = 30, retention = 345600 }
    payouts       = { fifo = true, visibility = 120, retention = 1209600 }
    outbox        = { fifo = false, visibility = 60, retention = 1209600 }
    notifications = { fifo = false, visibility = 60, retention = 345600 }
  }
}

resource "aws_sqs_queue" "dlq" {
  for_each                    = local.queues
  name                        = each.value.fifo ? "${var.name}-${each.key}-dlq.fifo" : "${var.name}-${each.key}-dlq"
  fifo_queue                  = each.value.fifo
  content_based_deduplication = each.value.fifo ? true : null
  message_retention_seconds   = 1209600
  kms_master_key_id           = var.kms_key_id
  tags                        = var.tags
}

resource "aws_sqs_queue" "this" {
  for_each                    = local.queues
  name                        = each.value.fifo ? "${var.name}-${each.key}.fifo" : "${var.name}-${each.key}"
  fifo_queue                  = each.value.fifo
  content_based_deduplication = each.value.fifo ? true : null
  deduplication_scope         = each.value.fifo ? "messageGroup" : null
  fifo_throughput_limit       = each.value.fifo ? "perMessageGroupId" : null
  visibility_timeout_seconds  = each.value.visibility
  message_retention_seconds   = each.value.retention
  receive_wait_time_seconds   = 20
  kms_master_key_id           = var.kms_key_id
  tags                        = var.tags

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq[each.key].arn
    maxReceiveCount     = 5
  })
}

resource "aws_sns_topic" "domain_events" {
  name              = "${var.name}-domain-events"
  kms_master_key_id = var.kms_key_id
  tags              = var.tags
}

# SNS → radha e njoftimeve (fan-out i parë; të tjerët shtohen sipas moduleve)
resource "aws_sns_topic_subscription" "notifications" {
  topic_arn            = aws_sns_topic.domain_events.arn
  protocol             = "sqs"
  endpoint             = aws_sqs_queue.this["notifications"].arn
  raw_message_delivery = true
}

data "aws_iam_policy_document" "notifications_from_sns" {
  statement {
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.this["notifications"].arn]
    principals {
      type        = "Service"
      identifiers = ["sns.amazonaws.com"]
    }
    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_sns_topic.domain_events.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "notifications" {
  queue_url = aws_sqs_queue.this["notifications"].id
  policy    = data.aws_iam_policy_document.notifications_from_sns.json
}

# Alarme: mesazhe në DLQ = diçka po dështon vazhdimisht
resource "aws_cloudwatch_metric_alarm" "dlq" {
  for_each            = local.queues
  alarm_name          = "${var.name}-${each.key}-dlq-not-empty"
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { QueueName = aws_sqs_queue.dlq[each.key].name }
  alarm_actions       = var.alarm_topic_arn == null ? [] : [var.alarm_topic_arn]
  tags                = var.tags
}
