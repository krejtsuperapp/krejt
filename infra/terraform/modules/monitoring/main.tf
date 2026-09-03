# Monitorim dhe siguri (§57, §71): një rënie duhet të mbërrijë me email para se ta raportojë
# një klient. Alarmet mbulojnë tri shtresat — ALB-ja (çfarë sheh përdoruesi), ECS-ja (a po
# ekzekutohet gjë) dhe baza (a ka frymë) — dhe të gjitha shkojnë te i njëjti kanal.

data "aws_caller_identity" "this" {}

# ------------------------------- kanali i alarmeve --------------------------------------
resource "aws_sns_topic" "alerts" {
  name = "${var.name}-alerts"
  tags = var.tags
}

# Abonimi me email kërkon konfirmim një herë: AWS-i dërgon një lidhje, dhe deri atëherë
# alarmet nuk mbërrijnë. Kjo nuk mund të automatizohet — është mbrojtje kundër spam-it.
resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

locals {
  actions = [aws_sns_topic.alerts.arn]
  alb_n   = var.alb_arn_suffix == null ? 0 : 1
  api_n   = var.api_service_name == null ? 0 : 1
  wrk_n   = var.worker_service_name == null ? 0 : 1
  cf_n    = var.centrifugo_service_name == null ? 0 : 1
}

# ------------------------------- ALB: çfarë sheh përdoruesi -----------------------------
# 5xx nga vetë ALB-ja: zakonisht "asnjë target i shëndetshëm" — pra API-ja nuk ekziston.
resource "aws_cloudwatch_metric_alarm" "alb_5xx" {
  count               = local.alb_n
  alarm_name          = "${var.name}-alb-5xx"
  alarm_description   = "ALB-ja kthen 5xx: zakonisht asnjë target i shëndetshëm pas saj."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_ELB_5XX_Count"
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 5
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { LoadBalancer = var.alb_arn_suffix }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

# 5xx nga aplikacioni: kodi ynë po dështon, jo infrastruktura.
resource "aws_cloudwatch_metric_alarm" "target_5xx" {
  count               = local.alb_n
  alarm_name          = "${var.name}-api-5xx"
  alarm_description   = "API-ja kthen 5xx: gabime të brendshme mbi pragun brenda 5 minutash."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = var.target_5xx_per_5min
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { LoadBalancer = var.alb_arn_suffix }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

# Vonesa p95: mesatarja fsheh; p95 tregon çfarë përjeton një në njëzet përdorues.
resource "aws_cloudwatch_metric_alarm" "latency_p95" {
  count               = local.alb_n
  alarm_name          = "${var.name}-api-latency-p95"
  alarm_description   = "Koha e përgjigjes p95 e API-së mbi pragun për 3 periudha rresht."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "TargetResponseTime"
  extended_statistic  = "p95"
  period              = 60
  evaluation_periods  = 3
  threshold           = var.latency_p95_seconds
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { LoadBalancer = var.alb_arn_suffix }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "api_unhealthy" {
  count               = local.alb_n
  alarm_name          = "${var.name}-api-unhealthy-hosts"
  alarm_description   = "Të paktën një target i API-së dështon kontrollin e shëndetit."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "UnHealthyHostCount"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  dimensions = {
    LoadBalancer = var.alb_arn_suffix
    TargetGroup  = var.api_target_group_arn_suffix
  }
  alarm_actions = local.actions
  ok_actions    = local.actions
  tags          = var.tags
}

# ------------------------------- ECS: a po ekzekutohet gjë -----------------------------
# Container Insights jep RunningTaskCount për shërbim. Zero për 3 minuta = shërbimi ka rënë,
# pavarësisht nga ALB-ja (worker-i nuk ka ALB fare, ndaj kjo është e vetmja sy për të).
resource "aws_cloudwatch_metric_alarm" "api_tasks" {
  count               = local.api_n
  alarm_name          = "${var.name}-api-no-tasks"
  alarm_description   = "Asnjë detyrë e API-së në ekzekutim."
  namespace           = "ECS/ContainerInsights"
  metric_name         = "RunningTaskCount"
  statistic           = "Minimum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  dimensions          = { ClusterName = var.cluster_name, ServiceName = var.api_service_name }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "worker_tasks" {
  count               = local.wrk_n
  alarm_name          = "${var.name}-worker-no-tasks"
  alarm_description   = "Asnjë detyrë e worker-it: radhët grumbullohen, njoftimet dhe pagesat ngecin."
  namespace           = "ECS/ContainerInsights"
  metric_name         = "RunningTaskCount"
  statistic           = "Minimum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  dimensions          = { ClusterName = var.cluster_name, ServiceName = var.worker_service_name }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "centrifugo_tasks" {
  count               = local.cf_n
  alarm_name          = "${var.name}-centrifugo-no-tasks"
  alarm_description   = "Asnjë detyrë e Centrifugo-s: aplikacionet bien te pyetja periodike."
  namespace           = "ECS/ContainerInsights"
  metric_name         = "RunningTaskCount"
  statistic           = "Minimum"
  period              = 60
  evaluation_periods  = 3
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  dimensions          = { ClusterName = var.cluster_name, ServiceName = var.centrifugo_service_name }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

# ------------------------------- Aurora: a ka frymë baza ------------------------------
resource "aws_cloudwatch_metric_alarm" "db_cpu" {
  alarm_name          = "${var.name}-db-cpu"
  alarm_description   = "CPU-ja e bazës mbi 80% për 10 minuta."
  namespace           = "AWS/RDS"
  metric_name         = "CPUUtilization"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { DBClusterIdentifier = var.aurora_cluster_id }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

# Aurora Serverless v2: ACU-të afër maksimumit do të thotë se baza po kërkon më shumë se sa lejohet.
resource "aws_cloudwatch_metric_alarm" "db_acu" {
  alarm_name          = "${var.name}-db-acu-high"
  alarm_description   = "Aurora po përdor mbi 90% të kapacitetit maksimal të lejuar (ACU)."
  namespace           = "AWS/RDS"
  metric_name         = "ACUUtilization"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 90
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { DBClusterIdentifier = var.aurora_cluster_id }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

resource "aws_cloudwatch_metric_alarm" "db_connections" {
  alarm_name          = "${var.name}-db-connections"
  alarm_description   = "Lidhje të hapura me bazën mbi 200: pool-i po rrjedh ose ngarkesa u rrit."
  namespace           = "AWS/RDS"
  metric_name         = "DatabaseConnections"
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 200
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  dimensions          = { DBClusterIdentifier = var.aurora_cluster_id }
  alarm_actions       = local.actions
  ok_actions          = local.actions
  tags                = var.tags
}

# ------------------------------- CloudTrail: kush bëri çfarë ----------------------------
# Historia 90-ditëshe e AWS-it mjafton për sot, por prodhimi duhet të mbajë një vit dhe ta
# ketë të pandryshueshme: kova pranon vetëm shkrim nga vetë shërbimi dhe nuk fshihet publikisht.
resource "aws_s3_bucket" "trail" {
  count  = var.cloudtrail_enabled ? 1 : 0
  bucket = "${var.name}-cloudtrail-${data.aws_caller_identity.this.account_id}"
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "trail" {
  count                   = var.cloudtrail_enabled ? 1 : 0
  bucket                  = aws_s3_bucket.trail[0].id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "trail" {
  count  = var.cloudtrail_enabled ? 1 : 0
  bucket = aws_s3_bucket.trail[0].id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "trail" {
  count  = var.cloudtrail_enabled ? 1 : 0
  bucket = aws_s3_bucket.trail[0].id
  rule {
    id     = "expire"
    status = "Enabled"
    filter {}
    expiration { days = var.cloudtrail_retention_days }
  }
}

data "aws_iam_policy_document" "trail" {
  count = var.cloudtrail_enabled ? 1 : 0
  statement {
    sid       = "AclCheck"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.trail[0].arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }
  statement {
    sid       = "Write"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.trail[0].arn}/AWSLogs/${data.aws_caller_identity.this.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
}

resource "aws_s3_bucket_policy" "trail" {
  count  = var.cloudtrail_enabled ? 1 : 0
  bucket = aws_s3_bucket.trail[0].id
  policy = data.aws_iam_policy_document.trail[0].json
}

resource "aws_cloudtrail" "this" {
  count                         = var.cloudtrail_enabled ? 1 : 0
  name                          = "${var.name}-trail"
  s3_bucket_name                = aws_s3_bucket.trail[0].id
  is_multi_region_trail         = true
  include_global_service_events = true
  enable_log_file_validation    = true
  tags                          = var.tags
  depends_on                    = [aws_s3_bucket_policy.trail]
}

# ------------------------------- GuardDuty ------------------------------------------------
resource "aws_guardduty_detector" "this" {
  count  = var.guardduty_enabled ? 1 : 0
  enable = true
  tags   = var.tags
}

# Gjetjet e GuardDuty-t shkojnë te i njëjti kanal si alarmet, që të mos kërkohen në konsolë.
resource "aws_cloudwatch_event_rule" "guardduty" {
  count       = var.guardduty_enabled ? 1 : 0
  name        = "${var.name}-guardduty-findings"
  description = "Gjetje të GuardDuty-t me ashpërsi të mesme ose të lartë."
  event_pattern = jsonencode({
    source      = ["aws.guardduty"]
    detail-type = ["GuardDuty Finding"]
    detail      = { severity = [{ numeric = [">=", 4] }] }
  })
  tags = var.tags
}

resource "aws_cloudwatch_event_target" "guardduty" {
  count = var.guardduty_enabled ? 1 : 0
  rule  = aws_cloudwatch_event_rule.guardduty[0].name
  arn   = aws_sns_topic.alerts.arn
}

data "aws_iam_policy_document" "alerts_topic" {
  statement {
    sid       = "Events"
    actions   = ["sns:Publish"]
    resources = [aws_sns_topic.alerts.arn]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com", "cloudwatch.amazonaws.com"]
    }
  }
}

resource "aws_sns_topic_policy" "alerts" {
  arn    = aws_sns_topic.alerts.arn
  policy = data.aws_iam_policy_document.alerts_topic.json
}
