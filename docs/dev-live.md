# Ngritja e dev-it në internet

Mjedisi dev ekziston tashmë në AWS: Aurora, Redis, radhët dhe sekretet u krijuan me
`terraform apply`. Ky dokument e bën të arritshëm nga jashtë, që aplikacioni të provohet nga
një telefon i vërtetë e jo vetëm nga laptopi.

**Çdo komandë ekzekutohet në makinën tënde.** Asnjë vlerë sekreti nuk shkruhet në repo.

## Sa kushton

NAT-i dhe ALB-ja janë të vetmet burime që paguhen edhe kur askush nuk i përdor: bashkë rreth
55–70 USD në muaj. Për këtë arsye buxheti i alarmit u ngrit në 120 USD. Aurora në dev ka
auto-pause, ndaj kur nuk përdoret nuk shpenzon.

Kur dev-i nuk të duhet më, `nat_enabled = false` dhe `alb_enabled = false` e kthejnë koston
pothuajse në zero pa e shkatërruar mjedisin.

## 1. Certifikata

ALB-ja nuk ngrihet pa të. Lëshohet në `eu-central-1`, sepse aty rri.

```bash
aws acm request-certificate --domain-name dev.krejt.app --validation-method DNS --region eu-central-1 --profile krejt-dev
```

Merr regjistrimin CNAME të validimit:

```bash
aws acm describe-certificate --certificate-arn <ARN> --region eu-central-1 --profile krejt-dev --query 'Certificate.DomainValidationOptions[0].ResourceRecord'
```

Shtoje te Cloudflare **me proxy të fikur** (renë gri): validimi i ACM-së nuk kalon përmes
proxy-t. Kur statusi bëhet `ISSUED`, vendos ARN-në te `envs/dev/terraform.tfvars`.

## 2. Vlerat e sekreteve

Vendosen nga konsola, siç përshkruhet te `docs/sekretet.md`, ose me `put-secret-value`.

Këto duhen doemos, përndryshe detyra nuk niset:

| Sekreti | Nga vjen |
| --- | --- |
| `krejt-dev/jwt` | `private_key_pem` gjenerohet lokalisht; `otp_pepper` varg i rastësishëm |
| `krejt-dev/mapbox-token` | token-i i backend-it te Mapbox |
| `krejt-dev/google-maps` | vendmbajtës nëse përdor Mapbox |
| `krejt-dev/infobip` | çelësi i API-së |
| `krejt-dev/payment-provider` | `secret_key` dhe `webhook_secret` nga Stripe |
| `krejt-dev/fcm` | JSON-i i llogarisë së shërbimit nga Firebase |
| `krejt-dev/centrifugo` | `api_key` dhe `token_hmac_secret_key`, të rastësishëm |
| `krejt-dev/posthog` | çelësi i projektit |
| `krejt-dev/sentry` | DSN i vërtetë; një i pavlefshëm e ndal nisjen |
| `krejt-dev/otel` dhe `postmark` | vendmbajtës derisa të përdoren |

Te tfvars plotëso edhe `infobip_base_url` me adresën personale të llogarisë tënde Infobip.

## 3. Ngritja

```bash
cd infra/terraform/envs/dev && terraform init
```

```bash
terraform plan -out dev.tfplan
```

Plani do të shtojë NAT-in, ALB-në dhe sekretin e ri të Mapbox-it. Lexoje para se ta zbatosh.

```bash
terraform apply dev.tfplan
```

## 4. Imazhi i parë

Deploy-i bëhet nga GitHub Actions me OIDC, pa çelësa. Merr ARN-në e rolit:

```bash
terraform output deploy_role_arn
```

Vendose te cilësimet e repo-s si variabël `AWS_DEPLOY_ROLE_ARN`. Pas bashkimit të parë në
`main`, rrjedha `deploy-dev` e ndërton imazhin dhe e shtyn në ECR.

Pastaj rrit numrin e detyrave te tfvars:

```
api_desired_count        = 1
worker_desired_count     = 1
centrifugo_desired_count = 1
```

```bash
terraform apply
```

## 5. DNS-ja

```bash
terraform output alb_dns_name
```

Te Cloudflare shto një CNAME `dev` → ai emër, **me proxy të ndezur**. Modaliteti TLS duhet
**Full (strict)**.

## 6. Provo

```bash
curl -fsS https://dev.krejt.app/healthz && echo
```

Kopjo `apps/dart-defines.example.json` te `apps/dart-defines.json` dhe plotëso vlerat. Ai skedar
nuk versionohet: mban token-in publik të hartës, ndaj rri vetëm te makina jote dhe nuk përfundon
as te historiku i terminalit.

Ndërto aplikacionin kundrejt tij:

```bash
cd apps/customer && flutter build apk --release --dart-define-from-file=../dart-defines.json
```

Meqë adresa është `https`, ndërtimi `--release` punon pa asnjë zbutje rrjeti.

Token-i `pk.` është ai publik i Mapbox-it; pa të, ekranet e udhëtimit vizatojnë skemën në vend
të hartës. Vetë ndërtimi kërkon token-in e shkarkimit te `~/.gradle/gradle.properties` — shih
[packages/krejt_map/README.md](../packages/krejt_map/README.md).

## Numrat e provës

Vetëm në dev, tre numra kyçen me kodin fiks **111111**, pa SMS dhe pa skadim (`DEV_TEST_PHONES`,
`DEV_TEST_OTP` te tfvars; serveri refuzon të niset me to jashtë development):

| Numri | Roli |
| --- | --- |
| `+38344100200` | administrator (`SUPER_ADMIN` në kyçje) |
| `+38344100201` | klient |
| `+38344100202` | shofer |

Aprovimi i shoferit në dev nuk kërkon dokumente (`documents_required = false`).

Paneli i administrimit hapet lokalisht kundrejt dev-it: `admin/.env.local` me
`KREJT_API_BASE_URL=https://dev.krejt.app`, pastaj `npm run dev --prefix admin`.

## Kyçja nga larg

Derisa sender-i te Infobip të aprovohet, SMS-ja nuk dërgohet dhe kyçja nga një telefon i largët
nuk mbyllet dot. Kodi shkruhet te log-u i detyrës në CloudWatch:

```bash
aws logs tail /ecs/krejt-dev/api --since 5m --profile krejt-dev --filter-pattern dev_only_code
```

Kjo punon vetëm nëse `sms_provider = "devlog"` te tfvars. Me `infobip` dhe një sender të
paaprovuar, kërkesa dështon pa dërguar asgjë.
