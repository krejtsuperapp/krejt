region      = "eu-central-1"
aws_profile = "krejt-staging"
environment = "staging"
name        = "krejt-staging"

assets_bucket_name = "krejt-assets-staging-7k2m9q"
media_bucket_name  = "krejt-media-staging-7k2m9q"

# ARN-ja e certifikatës për staging.krejt.app, e lëshuar në eu-central-1.
# Pa të, ALB-ja nuk ngrihet. Plotësohet pas hapit 2 të runbook-ut.
acm_certificate_arn = ""

# Rriten në 1 pasi imazhi i parë të jetë në ECR (hapi 5 i runbook-ut).
# Vlerat vlejnë vetëm kur shërbimi krijohet (ignore_changes); pas imazhit të parë numrin e
# ngre `aws ecs update-service` dhe autoscaling-u.
api_desired_count        = 1
worker_desired_count     = 1
centrifugo_desired_count = 1

monthly_budget_usd = 200
alert_email        = "krejtsuperapp@gmail.com"

# Adresa publike e API-së dhe ofruesit që zgjedh ky mjedis.
public_base_url = "https://staging.krejt.app"
maps_provider   = "mapbox" # google | mapbox

# Ofruesit e jashtëm që ende s'kanë llogari (Infobip, Stripe, PostHog) mbeten devlog në staging:
# OTP-ja shkruhet te CloudWatch, pagesat nuk kalojnë vetvetiu, ngjarjet vetëm logohen. Kthehen
# te ofruesi i vërtetë ditën që sekreti i tij vendoset. Në prodhim devlog refuzohet nga serveri.
sms_provider       = "devlog"
payment_provider   = "devlog"
analytics_provider = "devlog"
push_provider      = "fcm" # sekreti fcm u kopjua nga dev-i (i njëjti projekt Firebase)
# Base URL personale e llogarisë Infobip; e gjen te paneli i tyre (jo https://api.infobip.com).
infobip_base_url = ""
infobip_sender   = "KREJT"
