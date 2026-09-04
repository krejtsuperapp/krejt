region             = "eu-central-1"
aws_profile        = "krejt-dev"
environment        = "dev"
name               = "krejt-dev"
assets_bucket_name = "krejt-assets-dev-7k2m9q"
media_bucket_name  = "krejt-media-dev-7k2m9q"
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
# Vetëm dev: shoferi i provës aprovohet pa gjashtë dokumente. Jashtë development serveri
# refuzon të niset me këtë vlerë.
documents_required = false

# Vetëm dev: ky numër kyçet me kodin fiks, pa SMS dhe pa skadim, dhe është administrator.
# Hiqet para fazës finale; jashtë development serveri refuzon të niset me të.
# Tre role, tre numra: administratori, klienti, shoferi. Kodi i njëjtë për të tre.
dev_test_phones       = ["+38344100200", "+38344100201", "+38344100202"]
dev_test_admin_phones = ["+38344100200"]
dev_test_otp          = "111111"
sms_provider          = "devlog"
# Base URL personale e llogarisë Infobip; e gjen te paneli i tyre (jo https://api.infobip.com).
infobip_base_url = "https://rk8n2y.api.infobip.com"
infobip_sender   = "KREJT"

# Certifikatën e krijon Terraform-i për domain_name; kjo mbetet vetëm nëse do të përdorësh një të lëshuar diku tjetër.
acm_certificate_arn = ""

# Nga një detyrë për shërbim: dev-i shërben për prova nga larg, jo për ngarkesë.
# Vlerat e mëposhtme vlejnë vetëm kur shërbimi krijohet: pas kësaj numrin e drejtojnë
# autoscaling-u dhe CI-ja, ndaj `ignore_changes` e mban Terraform-in jashtë.
api_min_count            = 1
api_max_count            = 6
api_desired_count        = 1
worker_desired_count     = 1
centrifugo_desired_count = 1

# Numri që bëhet administratori i parë. Vepron një herë: pasi një administrator ekziston,
# ky cilësim nuk bën më asgjë.
bootstrap_admin_phone = "+38344123456"


# Certifikata e paneleve u lëshua (ISSUED); lidhet me listener-in HTTPS.
panel_cert_ready = true

# Identiteti i operatorit te Kushtet dhe Politika e privatësisë (dokumentet i shërben API-ja).
legal_entity  = "KREJT SUPER APP L.L.C"
legal_address = "Sylejman Vuqiterna 23, Kalabri, Prishtinë"
legal_email   = "info@krejt.app"
