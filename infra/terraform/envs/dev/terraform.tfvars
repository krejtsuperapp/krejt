region             = "eu-central-1"
aws_profile        = "krejt-dev"
environment        = "dev"
name               = "krejt-dev"
assets_bucket_name = "krejt-assets-dev-7k2m9q"
nat_enabled        = true
alb_enabled        = true
monthly_budget_usd = 120 # NAT dhe ALB e ngrenë koston bazë
alert_email        = "krejtsuperapp@gmail.com"

# Adresa publike e API-së dhe ofruesit që zgjedh ky mjedis.
public_base_url = "https://dev.krejt.app"
maps_provider   = "mapbox" # google | mapbox

# SMS-ja mbetet devlog derisa sender-i te Infobip të aprovohet: me një sender të paaprovuar
# kërkesa dështon pa dërguar asgjë, dhe askush nuk kyçet dot. Me devlog kodi shkruhet te
# CloudWatch, ndaj kyçja nga larg funksionon. Ndërroje në "infobip" ditën e aprovimit.
sms_provider = "devlog"
# Base URL personale e llogarisë Infobip; e gjen te paneli i tyre (jo https://api.infobip.com).
infobip_base_url = "https://rk8n2y.api.infobip.com"
infobip_sender   = "KREJT"

# ARN-ja e certifikatës për dev.krejt.app, e lëshuar në eu-central-1. ALB-ja nuk ngrihet pa të.
acm_certificate_arn = ""

# Rriten në 1 pasi imazhi i parë të jetë në ECR.
api_desired_count        = 0
worker_desired_count     = 0
centrifugo_desired_count = 0
