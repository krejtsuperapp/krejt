output "bucket_name" { value = aws_s3_bucket.media.bucket }
output "bucket_arn" { value = aws_s3_bucket.media.arn }
output "distribution_id" { value = one(aws_cloudfront_distribution.media[*].id) }
output "distribution_domain" { value = one(aws_cloudfront_distribution.media[*].domain_name) }
# Baza publike e imazheve (MEDIA_BASE_URL e backend-it): domeni i vetin nëse ka, ndryshe ai i
# CloudFront-it; "" kur CloudFront-i është i fikur — atëherë mjedisi vendos bazën e API-së.
output "base_url" {
  value = !var.cloudfront_enabled ? "" : (length(var.aliases) == 0 ? "https://${aws_cloudfront_distribution.media[0].domain_name}" : "https://${var.aliases[0]}")
}
