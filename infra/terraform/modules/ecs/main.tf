# -----------------------------------------------------------------------------
# KREJT — ECS Fargate (§43): cluster, ECR, rolet IAM, log groups, task definitions dhe
# (kur alb_enabled = true) ALB + tri shërbimet:
#   api        — backend Go (ALB /api/*), port 8080, health /healthz
#   worker     — konsumatorët e SQS (pa ALB)
#   centrifugo — realtime WebSocket (ALB /connection/*), port 8000, health /health
# desired_count fillon në 0 derisa të ekzistojë imazhi i parë në ECR — asnjë aplikacion i rremë.
# Në dev, alb_enabled = false derisa backend-i të ketë imazh (kursen ~20 $/muaj bosh).
# -----------------------------------------------------------------------------
data "aws_region" "this" {}
data "aws_caller_identity" "this" {}

locals {
  alb_n = var.alb_enabled ? 1 : 0
}

# ------------------------------- cluster --------------------------------------
resource "aws_ecs_cluster" "this" {
  name = "${var.name}-cluster"
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
  tags = var.tags
}

resource "aws_ecs_cluster_capacity_providers" "this" {
  cluster_name       = aws_ecs_cluster.this.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]
  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
}

# ------------------------------- ECR ------------------------------------------
resource "aws_ecr_repository" "this" {
  for_each             = toset(["api", "worker"])
  name                 = "${var.name}/${each.value}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = !var.protect
  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = var.kms_key_arn
  }
  image_scanning_configuration {
    scan_on_push = true
  }
  tags = var.tags
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each   = aws_ecr_repository.this
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "mbaj 30 imazhet e fundit"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 30 }
      action       = { type = "expire" }
    }]
  })
}

# ------------------------------- security groups ------------------------------
resource "aws_security_group" "alb" {
  count       = local.alb_n
  name        = "${var.name}-alb-sg"
  description = "ALB - HTTP/HTTPS from internet (behind Cloudflare)"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-alb-sg" })
}

resource "aws_vpc_security_group_ingress_rule" "alb_http" {
  count             = local.alb_n
  security_group_id = aws_security_group.alb[0].id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  to_port           = 80
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "alb_https" {
  count             = local.alb_n
  security_group_id = aws_security_group.alb[0].id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "alb_all" {
  count             = local.alb_n
  security_group_id = aws_security_group.alb[0].id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_security_group" "app" {
  name        = "${var.name}-app-sg"
  description = "ECS tasks - ingress only from ALB; egress via NAT"
  vpc_id      = var.vpc_id
  tags        = merge(var.tags, { Name = "${var.name}-app-sg" })
}

resource "aws_vpc_security_group_ingress_rule" "app_from_alb" {
  count                        = local.alb_n
  security_group_id            = aws_security_group.app.id
  referenced_security_group_id = aws_security_group.alb[0].id
  from_port                    = 8000
  to_port                      = 8080
  ip_protocol                  = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "app_from_app" {
  security_group_id            = aws_security_group.app.id
  referenced_security_group_id = aws_security_group.app.id
  ip_protocol                  = "-1"
}

resource "aws_vpc_security_group_egress_rule" "app_all" {
  security_group_id = aws_security_group.app.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

# ------------------------------- ALB (opsional) -------------------------------
resource "aws_lb" "this" {
  count                      = local.alb_n
  name                       = "${var.name}-alb"
  load_balancer_type         = "application"
  internal                   = false
  security_groups            = [aws_security_group.alb[0].id]
  subnets                    = var.public_subnet_ids
  drop_invalid_header_fields = true
  enable_deletion_protection = var.protect
  idle_timeout               = 120
  tags                       = var.tags
}

resource "aws_lb_target_group" "api" {
  count                = local.alb_n
  name                 = "${var.name}-api-tg"
  port                 = 8080
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = 20
  health_check {
    path                = "/healthz"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
    matcher             = "200"
  }
  tags = var.tags
}

resource "aws_lb_target_group" "centrifugo" {
  count                = local.alb_n
  name                 = "${var.name}-rt-tg"
  port                 = 8000
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = 20
  health_check {
    path                = "/health"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
    matcher             = "200"
  }
  tags = var.tags
}

# HTTP:80 tani; HTTPS:443 me ACM shtohet sapo domain-i krejt.app të jetë në Cloudflare (var.acm_certificate_arn).
resource "aws_lb_listener" "http" {
  count             = local.alb_n
  load_balancer_arn = aws_lb.this[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = var.acm_certificate_arn == null ? "fixed-response" : "redirect"
    dynamic "fixed_response" {
      for_each = var.acm_certificate_arn == null ? [1] : []
      content {
        content_type = "application/json"
        message_body = "{\"error\":{\"code\":\"NOT_FOUND\",\"message_key\":\"errors.not_found\"}}"
        status_code  = "404"
      }
    }
    dynamic "redirect" {
      for_each = var.acm_certificate_arn == null ? [] : [1]
      content {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  }
}

resource "aws_lb_listener" "https" {
  count             = var.alb_enabled && var.acm_certificate_arn != null ? 1 : 0
  load_balancer_arn = aws_lb.this[0].arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.acm_certificate_arn

  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "application/json"
      message_body = "{\"error\":{\"code\":\"NOT_FOUND\",\"message_key\":\"errors.not_found\"}}"
      status_code  = "404"
    }
  }
}

locals {
  listener_arn = var.alb_enabled ? (var.acm_certificate_arn == null ? aws_lb_listener.http[0].arn : aws_lb_listener.https[0].arn) : null
}

resource "aws_lb_listener_rule" "api" {
  count        = local.alb_n
  listener_arn = local.listener_arn
  priority     = 10
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api[0].arn
  }
  condition {
    path_pattern { values = ["/api/*", "/healthz"] }
  }
}

resource "aws_lb_listener_rule" "centrifugo" {
  count        = local.alb_n
  listener_arn = local.listener_arn
  priority     = 20
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.centrifugo[0].arn
  }
  condition {
    path_pattern { values = ["/connection/*", "/health"] }
  }
}

# ------------------------------- log groups -----------------------------------
resource "aws_cloudwatch_log_group" "svc" {
  for_each          = toset(["api", "worker", "centrifugo"])
  name              = "/krejt/${var.name}/${each.value}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

# ------------------------------- IAM ------------------------------------------
data "aws_iam_policy_document" "ecs_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# roli i ekzekutimit: tërheq imazhin, shkruan log-e, lexon sekretet për env
resource "aws_iam_role" "exec" {
  name               = "${var.name}-ecs-exec"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
  tags               = var.tags
}

resource "aws_iam_role_policy_attachment" "exec_managed" {
  role       = aws_iam_role.exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "exec_secrets" {
  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = var.secret_arns
  }
  statement {
    actions   = ["kms:Decrypt"]
    resources = [var.kms_key_arn]
  }
}

resource "aws_iam_role_policy" "exec_secrets" {
  name   = "read-secrets"
  role   = aws_iam_role.exec.id
  policy = data.aws_iam_policy_document.exec_secrets.json
}

# roli i task-ut: çfarë bën aplikacioni në runtime (S3, SQS, SNS, sekrete, KMS)
resource "aws_iam_role" "task" {
  name               = "${var.name}-ecs-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
  tags               = var.tags
}

data "aws_iam_policy_document" "task" {
  statement {
    sid       = "Assets"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
    resources = [var.assets_bucket_arn, "${var.assets_bucket_arn}/*"]
  }
  statement {
    sid       = "Queues"
    actions   = ["sqs:SendMessage", "sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes", "sqs:ChangeMessageVisibility"]
    resources = var.queue_arns
  }
  statement {
    sid       = "Events"
    actions   = ["sns:Publish"]
    resources = [var.domain_events_topic_arn]
  }
  statement {
    sid       = "Secrets"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = var.secret_arns
  }
  statement {
    sid       = "Kms"
    actions   = ["kms:Decrypt", "kms:GenerateDataKey"]
    resources = [var.kms_key_arn]
  }
  statement {
    sid       = "Telemetry"
    actions   = ["xray:PutTraceSegments", "xray:PutTelemetryRecords", "cloudwatch:PutMetricData"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "task" {
  name   = "runtime"
  role   = aws_iam_role.task.id
  policy = data.aws_iam_policy_document.task.json
}

# ------------------------------- task definitions -----------------------------
locals {
  ecr_base = "${data.aws_caller_identity.this.account_id}.dkr.ecr.${data.aws_region.this.name}.amazonaws.com/${var.name}"

  # APP_ENV i backend-it: development | staging | production (emri i mjedisit AWS mbetet dev/staging/prod)
  app_env = lookup({ dev = "development", staging = "staging", prod = "production" }, var.environment, var.environment)

  common_env = [
    { name = "APP_ENV", value = local.app_env },
    { name = "AWS_REGION", value = data.aws_region.this.name },
    { name = "S3_ASSETS_BUCKET", value = var.assets_bucket_name },
    { name = "SNS_DOMAIN_EVENTS_TOPIC_ARN", value = var.domain_events_topic_arn },
    { name = "DB_WRITER_HOST", value = var.aurora_writer_endpoint },
    { name = "DB_READER_HOST", value = var.aurora_reader_endpoint },
    { name = "DB_NAME", value = "krejt" },
    { name = "REDIS_HOST", value = var.redis_endpoint },
    { name = "REDIS_TLS", value = "true" },
    { name = "CENTRIFUGO_API_URL", value = "http://centrifugo.${var.name}.local:8000/api" },
    { name = "PUBLIC_BASE_URL", value = var.public_base_url },
    # Ofruesit zgjidhen këtu e jo nga vlerat e paracaktuara të kodit: një mjedis i vërtetë
    # nuk duhet të varet nga ajo çka ndodh të jetë default.
    { name = "MAPS_PROVIDER", value = var.maps_provider },
    { name = "SMS_PROVIDER", value = var.sms_provider },
    { name = "PAYMENT_PROVIDER", value = var.payment_provider },
    { name = "PUSH_PROVIDER", value = var.push_provider },
    { name = "ANALYTICS_PROVIDER", value = var.analytics_provider },
    { name = "INFOBIP_BASE_URL", value = var.infobip_base_url },
    { name = "INFOBIP_SENDER", value = var.infobip_sender },
    { name = "OTEL_EXPORTER_OTLP_ENDPOINT", value = var.otlp_endpoint },
    { name = "OTEL_TRACES_SAMPLER_ARG", value = "0.2" },
  ]
  queue_env = [for k, u in var.queue_urls : { name = "SQS_${upper(k)}_QUEUE_URL", value = u }]

  # Sekretet e aplikacionit (vlerat futen me dorë në Secrets Manager; task-et nuk nisen pa to):
  #  - krejt-<env>/jwt: JSON { "private_key_pem": "...", "otp_pepper": "..." }
  #  - krejt-<env>/google-maps, krejt-<env>/mapbox-token, krejt-<env>/infobip: vlera e thjeshtë (çelësi); krejt-<env>/fcm: JSON-i i llogarisë së shërbimit
  common_secrets = [
    { name = "DB_CREDENTIALS_JSON", valueFrom = var.aurora_master_secret_arn },
    { name = "REDIS_AUTH", valueFrom = var.redis_auth_secret_arn },
    { name = "JWT_PRIVATE_KEY", valueFrom = "${var.app_secret_arns["jwt"]}:private_key_pem::" },
    { name = "OTP_PEPPER", valueFrom = "${var.app_secret_arns["jwt"]}:otp_pepper::" },
    # Të dy ofruesit e hartave janë të lidhur; MAPS_PROVIDER vendos cili përdoret (§46).
    { name = "GOOGLE_MAPS_KEY", valueFrom = var.app_secret_arns["google-maps"] },
    { name = "MAPBOX_TOKEN", valueFrom = var.app_secret_arns["mapbox-token"] },
    { name = "INFOBIP_API_KEY", valueFrom = var.app_secret_arns["infobip"] },
    { name = "FCM_SERVICE_ACCOUNT_JSON", valueFrom = var.app_secret_arns["fcm"] },
    { name = "STRIPE_SECRET_KEY", valueFrom = "${var.app_secret_arns["payment-provider"]}:secret_key::" },
    { name = "STRIPE_WEBHOOK_SECRET", valueFrom = "${var.app_secret_arns["payment-provider"]}:webhook_secret::" },
    { name = "SENTRY_DSN", valueFrom = var.app_secret_arns["sentry"] },
    { name = "POSTHOG_KEY", valueFrom = var.app_secret_arns["posthog"] },
    { name = "OTEL_EXPORTER_OTLP_HEADERS", valueFrom = var.app_secret_arns["otel"] },
    { name = "CENTRIFUGO_API_KEY", valueFrom = "${var.centrifugo_secret_arn}:api_key::" },
    { name = "CENTRIFUGO_TOKEN_HMAC_SECRET", valueFrom = "${var.centrifugo_secret_arn}:token_hmac_secret_key::" },
  ]
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.name}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  container_definitions = jsonencode([{
    name         = "api"
    image        = "${local.ecr_base}/api:${var.image_tag}"
    essential    = true
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment  = concat(local.common_env, local.queue_env, [{ name = "HTTP_PORT", value = "8080" }])
    secrets      = local.common_secrets
    healthCheck = {
      command     = ["CMD-SHELL", "wget -qO- http://localhost:8080/healthz || exit 1"]
      interval    = 15
      timeout     = 5
      retries     = 3
      startPeriod = 20
    }
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.svc["api"].name
        awslogs-region        = data.aws_region.this.name
        awslogs-stream-prefix = "api"
      }
    }
  }])
  tags = var.tags
}

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.name}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.worker_cpu
  memory                   = var.worker_memory
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  container_definitions = jsonencode([{
    name        = "worker"
    image       = "${local.ecr_base}/worker:${var.image_tag}"
    essential   = true
    environment = concat(local.common_env, local.queue_env)
    secrets     = local.common_secrets
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.svc["worker"].name
        awslogs-region        = data.aws_region.this.name
        awslogs-stream-prefix = "worker"
      }
    }
  }])
  tags = var.tags
}

resource "aws_ecs_task_definition" "centrifugo" {
  family                   = "${var.name}-centrifugo"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 512
  memory                   = 1024
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  container_definitions = jsonencode([{
    name         = "centrifugo"
    image        = "centrifugo/centrifugo:${var.centrifugo_version}"
    essential    = true
    portMappings = [{ containerPort = 8000, protocol = "tcp" }]
    command      = ["centrifugo", "--health", "--admin=false", "--engine=redis", "--redis_address=rediss://${var.redis_endpoint}:6379", "--redis_cluster_address=rediss://${var.redis_endpoint}:6379", "--allowed_origins=https://krejt.app,https://*.krejt.app"]
    environment  = [{ name = "CENTRIFUGO_PORT", value = "8000" }]
    secrets = [
      { name = "CENTRIFUGO_TOKEN_HMAC_SECRET_KEY", valueFrom = "${var.centrifugo_secret_arn}:token_hmac_secret_key::" },
      { name = "CENTRIFUGO_API_KEY", valueFrom = "${var.centrifugo_secret_arn}:api_key::" },
      { name = "CENTRIFUGO_REDIS_PASSWORD", valueFrom = var.redis_auth_secret_arn },
    ]
    healthCheck = {
      command     = ["CMD-SHELL", "wget -qO- http://localhost:8000/health || exit 1"]
      interval    = 15
      timeout     = 5
      retries     = 3
      startPeriod = 15
    }
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.svc["centrifugo"].name
        awslogs-region        = data.aws_region.this.name
        awslogs-stream-prefix = "centrifugo"
      }
    }
  }])
  tags = var.tags
}

# ------------------------------- services -------------------------------------
resource "aws_ecs_service" "api" {
  count                             = local.alb_n
  name                              = "${var.name}-api"
  cluster                           = aws_ecs_cluster.this.id
  task_definition                   = aws_ecs_task_definition.api.arn
  desired_count                     = var.api_desired_count
  launch_type                       = "FARGATE"
  platform_version                  = "LATEST"
  health_check_grace_period_seconds = 30
  enable_execute_command            = true
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.app.id]
    assign_public_ip = false
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.api[0].arn
    container_name   = "api"
    container_port   = 8080
  }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
  depends_on                         = [aws_lb_listener_rule.api]
  tags                               = var.tags
  lifecycle { ignore_changes = [desired_count, task_definition] } # deploy-i nga CI regjistron revizione të reja
}

resource "aws_ecs_service" "worker" {
  name                   = "${var.name}-worker"
  cluster                = aws_ecs_cluster.this.id
  task_definition        = aws_ecs_task_definition.worker.arn
  desired_count          = var.worker_desired_count
  launch_type            = "FARGATE"
  enable_execute_command = true
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.app.id]
    assign_public_ip = false
  }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
  tags = var.tags
  lifecycle { ignore_changes = [desired_count, task_definition] }
}

# DNS privat brenda VPC-së (§43): api/worker publikojnë te http://centrifugo.<name>.local:8000/api
resource "aws_service_discovery_private_dns_namespace" "this" {
  name        = "${var.name}.local"
  vpc         = var.vpc_id
  description = "KREJT ${var.name} — zbulim i brendshëm i shërbimeve"
  tags        = var.tags
}

resource "aws_service_discovery_service" "centrifugo" {
  name = "centrifugo"
  dns_config {
    namespace_id   = aws_service_discovery_private_dns_namespace.this.id
    routing_policy = "MULTIVALUE"
    dns_records {
      ttl  = 10
      type = "A"
    }
  }
  health_check_custom_config { failure_threshold = 1 }
  tags = var.tags
}

resource "aws_ecs_service" "centrifugo" {
  count                             = local.alb_n
  name                              = "${var.name}-centrifugo"
  cluster                           = aws_ecs_cluster.this.id
  task_definition                   = aws_ecs_task_definition.centrifugo.arn
  desired_count                     = var.centrifugo_desired_count
  launch_type                       = "FARGATE"
  health_check_grace_period_seconds = 30
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.app.id]
    assign_public_ip = false
  }
  load_balancer {
    target_group_arn = aws_lb_target_group.centrifugo[0].arn
    container_name   = "centrifugo"
    container_port   = 8000
  }
  service_registries {
    registry_arn = aws_service_discovery_service.centrifugo.arn
  }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
  depends_on = [aws_lb_listener_rule.centrifugo]
  tags       = var.tags
  lifecycle { ignore_changes = [desired_count] }
}

# ------------------------------- autoscaling (api) ----------------------------
resource "aws_appautoscaling_target" "api" {
  count              = local.alb_n
  service_namespace  = "ecs"
  resource_id        = "service/${aws_ecs_cluster.this.name}/${aws_ecs_service.api[0].name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = var.api_min_count
  max_capacity       = var.api_max_count
}

resource "aws_appautoscaling_policy" "api_cpu" {
  count              = local.alb_n
  name               = "${var.name}-api-cpu"
  policy_type        = "TargetTrackingScaling"
  service_namespace  = aws_appautoscaling_target.api[0].service_namespace
  resource_id        = aws_appautoscaling_target.api[0].resource_id
  scalable_dimension = aws_appautoscaling_target.api[0].scalable_dimension
  target_tracking_scaling_policy_configuration {
    target_value       = 60
    scale_in_cooldown  = 120
    scale_out_cooldown = 60
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
  }
}
