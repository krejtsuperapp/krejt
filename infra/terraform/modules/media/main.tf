# -----------------------------------------------------------------------------
# KREJT — imazhet publike (§43): logot/kopertinat e vendeve, imazhet e produkteve, fotot e
# profilit. Bucket i veçantë nga asetet private (dokumentet e shoferëve rrinë te `storage`):
# ngarkim me URL të nënshkruar nga backend-i, lexim vetëm përmes CloudFront-it (OAC), që
# imazhet të kenë URL të qëndrueshme dhe cache, pa e bërë kurrë bucket-in publik.
# -----------------------------------------------------------------------------
resource "aws_s3_bucket" "media" {
  bucket = var.bucket_name
  tags   = var.tags
}

resource "aws_s3_bucket_public_access_block" "media" {
  bucket                  = aws_s3_bucket.media.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "media" {
  bucket = aws_s3_bucket.media.id
  versioning_configuration {
    status = "Enabled"
  }
}

# SSE-S3, jo KMS: përmbajtja është publike sipas natyrës (shfaqet te çdo klient), dhe leximi nga
# CloudFront-i me KMS do të kërkonte grant të veçantë në politikën e çelësit pa fituar asgjë.
resource "aws_s3_bucket_server_side_encryption_configuration" "media" {
  bucket = aws_s3_bucket.media.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "media" {
  bucket = aws_s3_bucket.media.id
  rule {
    id     = "expire-old-versions"
    status = "Enabled"
    filter {}
    noncurrent_version_expiration {
      noncurrent_days = 30
    }
    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# PUT vjen nga shfletuesi (panelet) me URL të nënshkruar; aplikacionet mobile s'kanë Origin.
resource "aws_s3_bucket_cors_configuration" "media" {
  bucket = aws_s3_bucket.media.id
  cors_rule {
    allowed_methods = ["GET", "HEAD", "PUT"]
    allowed_origins = var.cors_origins
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3600
  }
}

# --- CloudFront me Origin Access Control -----------------------------------------
resource "aws_cloudfront_origin_access_control" "media" {
  count                             = var.cloudfront_enabled ? 1 : 0
  name                              = "${var.bucket_name}-oac"
  description                       = "KREJT media: vetëm CloudFront-i lexon nga bucket-i"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

locals {
  origin_id = "s3-${var.bucket_name}"
  # Politika të menaxhuara nga AWS: CachingOptimized (gzip/brotli, TTL sipas Cache-Control) dhe
  # CORS-S3Origin (kalon Origin te S3, që CORS-i të punojë për canvas/fetch nga panelet).
  cache_policy_id          = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  origin_request_policy_id = "88a5eaf4-2fd4-4709-b370-b4c650ea3fcf"
}

resource "aws_cloudfront_distribution" "media" {
  count           = var.cloudfront_enabled ? 1 : 0
  enabled         = true
  comment         = "KREJT media (${var.bucket_name})"
  price_class     = "PriceClass_100" # Evropë + Amerikë e Veriut: mjafton për Kosovën, kushton më pak
  http_version    = "http2and3"
  is_ipv6_enabled = true
  aliases         = var.aliases

  origin {
    domain_name              = aws_s3_bucket.media.bucket_regional_domain_name
    origin_id                = local.origin_id
    origin_access_control_id = aws_cloudfront_origin_access_control.media[0].id
  }

  default_cache_behavior {
    allowed_methods          = ["GET", "HEAD", "OPTIONS"]
    cached_methods           = ["GET", "HEAD"]
    target_origin_id         = local.origin_id
    viewer_protocol_policy   = "redirect-to-https"
    compress                 = true
    cache_policy_id          = local.cache_policy_id
    origin_request_policy_id = local.origin_request_policy_id
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  # Pa domen të vetin: certifikata e CloudFront-it. Me `aliases` duhet certifikatë ACM në us-east-1.
  viewer_certificate {
    cloudfront_default_certificate = length(var.aliases) == 0
    acm_certificate_arn            = length(var.aliases) == 0 ? null : var.acm_certificate_arn_us_east_1
    ssl_support_method             = length(var.aliases) == 0 ? null : "sni-only"
    minimum_protocol_version       = length(var.aliases) == 0 ? "TLSv1" : "TLSv1.2_2021"
  }

  tags = var.tags
}

data "aws_iam_policy_document" "media_read" {
  count = var.cloudfront_enabled ? 1 : 0
  statement {
    sid       = "CloudFrontRead"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.media.arn}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.media[0].arn]
    }
  }
}

resource "aws_s3_bucket_policy" "media" {
  count  = var.cloudfront_enabled ? 1 : 0
  bucket = aws_s3_bucket.media.id
  policy = data.aws_iam_policy_document.media_read[0].json

  depends_on = [aws_s3_bucket_public_access_block.media]
}
