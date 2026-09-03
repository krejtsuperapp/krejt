# CI/CD (§70, §72): GitHub Actions hyn në AWS me OIDC (asnjë çelës i përhershëm), me një rol të kufizuar:
# shtyn imazhe në ECR-in e mjedisit, regjistron revizione task-esh dhe ripërtërin shërbimet ECS. Asgjë tjetër.

data "aws_caller_identity" "this" {}

resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1", "1c58a3a8518e8759bf075b76b750d4f2df264fcd"]
  tags            = var.tags
}

locals {
  # Organizata ka ndezur identifikuesit e pandryshueshëm, ndaj GitHub-i e vulos token-in me
  # `owner@id/repo@id` e jo me emrat. Ajo formë mbijeton riemërtimet dhe nuk mund të rimerret
  # nga dikush që regjistron emrin e lirë më vonë, ndaj është edhe më e sigurt se emri.
  # Të dyja format lejohen: cilësimi mund të ndërrohet pa e prishur deploy-in.
  allowed_subjects = compact([
    "repo:${var.github_repo}:environment:${var.deploy_environment}",
    var.github_repo_id == "" ? "" : "repo:${var.github_repo_id}:environment:${var.deploy_environment}",
  ])
}

data "aws_iam_policy_document" "assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      # Puna e deploy-it deklaron `environment:`, ndaj GitHub-i e vulos token-in me mjedisin
      # e jo me degën. Ky është lidhje më e ngushtë se dega: një push i thjeshtë nuk e merr dot
      # rolin — duhet të kalojë nga mjedisi, që mund të ketë edhe miratim njeriu.
      values = local.allowed_subjects
    }
  }
}

resource "aws_iam_role" "deploy" {
  name               = "${var.name}-github-deploy"
  assume_role_policy = data.aws_iam_policy_document.assume.json
  tags               = var.tags
}

data "aws_iam_policy_document" "deploy" {
  statement {
    sid       = "EcrAuth"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid = "EcrPush"
    actions = [
      "ecr:BatchCheckLayerAvailability", "ecr:CompleteLayerUpload", "ecr:InitiateLayerUpload", "ecr:PutImage",
      "ecr:UploadLayerPart", "ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer", "ecr:DescribeImages", "ecr:DescribeRepositories",
    ]
    resources = var.ecr_repository_arns
  }
  statement {
    sid       = "EcsDescribe"
    actions   = ["ecs:DescribeTaskDefinition", "ecs:DescribeServices", "ecs:ListTaskDefinitions"]
    resources = ["*"]
  }
  statement {
    sid       = "EcsRegister"
    actions   = ["ecs:RegisterTaskDefinition"]
    resources = ["*"]
  }
  statement {
    sid       = "EcsUpdate"
    actions   = ["ecs:UpdateService"]
    resources = ["arn:aws:ecs:${var.region}:${data.aws_caller_identity.this.account_id}:service/${var.cluster_name}/${var.name}-*"]
  }
  statement {
    sid       = "PassTaskRoles"
    actions   = ["iam:PassRole"]
    resources = [var.task_role_arn, var.exec_role_arn]
    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "deploy" {
  name   = "deploy"
  role   = aws_iam_role.deploy.id
  policy = data.aws_iam_policy_document.deploy.json
}
