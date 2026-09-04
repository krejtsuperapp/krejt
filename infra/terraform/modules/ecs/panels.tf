# -----------------------------------------------------------------------------
# Panelet (Next.js në Fargate): Operacionet (admin) dhe partneri. Të dyja pas të njëjtës ALB,
# të ndara sipas host-it (admin.<mjedis>.krejt.app, partner.<mjedis>.krejt.app). Rregullat e
# host-it kanë përparësi para atyre të shtegut, sepse paneli ka rrugët e veta nën /api/*
# (proxy-i drejt API-së dhe kyçja) që nuk duhet t'i kapë rregulli i API-së.
#
# Mbrojtja: paneli kërkon kyçje me OTP dhe rol stafi; Cloudflare-i mbetet përpara (proxy i
# ndezur). Cloudflare Access (një shtresë e dytë, me email) shtohet nga pronari te Cloudflare.
# -----------------------------------------------------------------------------
locals {
  # emri i panelit → host-i; bosh kur mjedisi nuk i publikon panelet
  panels = local.alb_n == 1 ? var.panel_hosts : {}
  # rendi i qëndrueshëm për përparësitë e rregullave (1, 2, …), para api-t (10)
  panel_order = { for i, k in sort(keys(local.panels)) : k => i + 1 }
}

resource "aws_vpc_security_group_ingress_rule" "app_panels_from_alb" {
  count                        = length(local.panels) > 0 ? 1 : 0
  security_group_id            = aws_security_group.app.id
  referenced_security_group_id = aws_security_group.alb[0].id
  from_port                    = 3000
  to_port                      = 3000
  ip_protocol                  = "tcp"
}

resource "aws_lb_target_group" "panel" {
  for_each             = local.panels
  name                 = "${var.name}-${each.key}-tg"
  port                 = 3000
  protocol             = "HTTP"
  target_type          = "ip"
  vpc_id               = var.vpc_id
  deregistration_delay = 20
  health_check {
    path                = "/"
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
    matcher             = "200-399" # faqja e hyrjes ridrejton; ridrejtimi është shëndet i mirë
  }
  tags = var.tags
}

resource "aws_lb_listener_rule" "panel" {
  for_each     = local.panels
  listener_arn = local.listener_arn
  priority     = local.panel_order[each.key]
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.panel[each.key].arn
  }
  condition {
    host_header { values = [each.value] }
  }
}

resource "aws_cloudwatch_log_group" "panel" {
  for_each          = local.panels
  name              = "/krejt/${var.name}/${each.key}"
  retention_in_days = var.log_retention_days
  kms_key_id        = var.kms_key_arn
  tags              = var.tags
}

resource "aws_ecs_task_definition" "panel" {
  for_each                 = local.panels
  family                   = "${var.name}-${each.key}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = 256
  memory                   = 512
  execution_role_arn       = aws_iam_role.exec.arn
  task_role_arn            = aws_iam_role.task.arn
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  container_definitions = jsonencode([{
    name         = each.key
    image        = "${local.ecr_base}/${each.key}:${var.image_tag}"
    essential    = true
    portMappings = [{ containerPort = 3000, protocol = "tcp" }]
    environment = [
      { name = "NODE_ENV", value = "production" },
      { name = "PORT", value = "3000" },
      { name = "HOSTNAME", value = "0.0.0.0" },
      # Paneli flet me API-në përmes adresës publike (Cloudflare → ALB), si çdo klient tjetër.
      { name = "KREJT_API_BASE_URL", value = var.public_base_url },
      { name = "KREJT_APP_VERSION", value = var.image_tag },
    ]
    healthCheck = {
      command     = ["CMD-SHELL", "wget -qO- --spider http://localhost:3000/ || exit 1"]
      interval    = 15
      timeout     = 5
      retries     = 3
      startPeriod = 20
    }
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.panel[each.key].name
        awslogs-region        = data.aws_region.this.name
        awslogs-stream-prefix = each.key
      }
    }
  }])
  tags = var.tags
}

resource "aws_ecs_service" "panel" {
  for_each                          = local.panels
  name                              = "${var.name}-${each.key}"
  cluster                           = aws_ecs_cluster.this.id
  task_definition                   = aws_ecs_task_definition.panel[each.key].arn
  desired_count                     = var.panel_desired_count
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
    target_group_arn = aws_lb_target_group.panel[each.key].arn
    container_name   = each.key
    container_port   = 3000
  }
  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
  depends_on                         = [aws_lb_listener_rule.panel]
  tags                               = var.tags
  lifecycle { ignore_changes = [desired_count, task_definition] } # deploy-i nga CI regjistron revizione të reja
}
