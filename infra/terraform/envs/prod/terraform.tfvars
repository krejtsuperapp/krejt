region      = "eu-central-1"
aws_profile = "krejt-prod"
environment = "prod"
name        = "krejt-prod"

assets_bucket_name = "krejt-assets-prod-7k2m9q"

# ARN-ja e certifikatës për api.krejt.app, e lëshuar në eu-central-1.
acm_certificate_arn = ""

# Prodhimi nis me nga dy detyra për shërbim, jo me një: një rinisje e vetme
# nuk duhet ta lërë API-në pa asnjë instancë.
api_desired_count        = 0
worker_desired_count     = 0
centrifugo_desired_count = 0

monthly_budget_usd = 600
alert_email        = "krejtsuperapp@gmail.com"
