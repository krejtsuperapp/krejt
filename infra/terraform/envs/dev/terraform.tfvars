region             = "eu-central-1"
aws_profile        = "krejt-dev"
environment        = "dev"
name               = "krejt-dev"
assets_bucket_name = "krejt-assets-dev-7k2m9q"
nat_enabled        = false # ndize kur backend-i të ketë imazhin e parë
alb_enabled        = false # ndize bashkë me nat_enabled
monthly_budget_usd = 60
alert_email        = "krejtsuperapp@gmail.com"

# Adresa publike e API-së dhe ofruesit që zgjedh ky mjedis.
public_base_url = "http://localhost:8080"
maps_provider   = "google" # google | mapbox
# Base URL personale e llogarisë Infobip; e gjen te paneli i tyre (jo https://api.infobip.com).
infobip_base_url = ""
infobip_sender   = "KREJT"
