# -----------------------------------------------------------------------------
# KREJT — KMS + Secrets Manager. Sekretet krijohen si "guaska" (pa vlerë):
# vlerat futen nga një njeri me `aws secretsmanager put-secret-value` ose nga konsola.
# Asnjë vlerë sekrete nuk kalon nëpër Terraform apo repo.
# -----------------------------------------------------------------------------
data "aws_caller_identity" "this" {}

data "aws_iam_policy_document" "kms" {
  statement {
    sid       = "AccountAdmin"
    actions   = ["kms:*"]
    resources = ["*"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.this.account_id}:root"]
    }
  }
  statement {
    sid       = "AllowAwsServices"
    actions   = ["kms:Encrypt", "kms:Decrypt", "kms:ReEncrypt*", "kms:GenerateDataKey*", "kms:DescribeKey", "kms:CreateGrant"]
    resources = ["*"]
    principals {
      type        = "Service"
      identifiers = ["rds.amazonaws.com", "elasticache.amazonaws.com", "secretsmanager.amazonaws.com", "sqs.amazonaws.com", "sns.amazonaws.com", "s3.amazonaws.com", "logs.${var.region}.amazonaws.com"]
    }
  }
}

resource "aws_kms_key" "this" {
  description             = "KREJT ${var.name} — çelësi kryesor i enkriptimit"
  enable_key_rotation     = true
  deletion_window_in_days = 30
  policy                  = data.aws_iam_policy_document.kms.json
  tags                    = var.tags
}

resource "aws_kms_alias" "this" {
  name          = "alias/${var.name}"
  target_key_id = aws_kms_key.this.key_id
}

resource "aws_secretsmanager_secret" "provider" {
  for_each                = toset(var.secret_names)
  name                    = "${var.name}/${each.value}"
  description             = "KREJT ${var.name} — ${each.value} (vlera futet manualisht, jo nga Terraform)"
  kms_key_id              = aws_kms_key.this.arn
  recovery_window_in_days = 7
  tags                    = var.tags
}
