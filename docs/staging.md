# Ngritja e staging-ut

Ky dokument e çon staging-un nga asgjë deri te një API që përgjigjet në
`https://staging.krejt.app`. Pas tij, aplikacionet ndërtohen kundrejt një serveri të vërtetë
dhe fillon numërimi i 30 ditëve pa incident P1 që kërkon §79.

**Çdo komandë ekzekutohet në makinën tënde.** Asnjë çelës, token apo fjalëkalim nuk duhet
t'i dërgohet askujt, as të shkruhet në repo. Terraform-i i krijon burimet; asgjë nuk krijohet
me dorë nga konsola, që gjendja të mbetet e përshkruar në kod (§43, §72).

## Para se të nisësh

Të duhen:

- një llogari AWS për staging, me profil lokal `krejt-staging` (`aws configure sso` ose çelësa
  të përkohshëm; profili qëndron vetëm te `~/.aws`)
- domeni `krejt.app` i menaxhuar nga Cloudflare
- Terraform ≥ 1.10 dhe AWS CLI v2

Verifiko profilin para gjithçkaje:

```bash
aws sts get-caller-identity --profile krejt-staging
```

## 1. Depoja e gjendjes

Terraform-i e ruan gjendjen në S3. Ky bucket krijohet një herë dhe nuk menaxhohet nga vetë
Terraform-i, sepse do t'i duhej vetes për ta krijuar veten.

```bash
aws s3api create-bucket --bucket krejt-tfstate-staging-7k2m9q --region eu-central-1 --create-bucket-configuration LocationConstraint=eu-central-1 --profile krejt-staging
```

```bash
aws s3api put-bucket-versioning --bucket krejt-tfstate-staging-7k2m9q --versioning-configuration Status=Enabled --profile krejt-staging
```

```bash
aws s3api put-public-access-block --bucket krejt-tfstate-staging-7k2m9q --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true --profile krejt-staging
```

## 2. Certifikata

ALB-ja nuk ngrihet pa certifikatë, ndaj kjo vjen para `apply`-t të parë. Certifikata duhet
lëshuar në `eu-central-1`, sepse aty rri ALB-ja.

```bash
aws acm request-certificate --domain-name staging.krejt.app --validation-method DNS --region eu-central-1 --profile krejt-staging
```

Merr emrin dhe vlerën e regjistrimit CNAME të validimit:

```bash
aws acm describe-certificate --certificate-arn <ARN> --region eu-central-1 --profile krejt-staging --query 'Certificate.DomainValidationOptions[0].ResourceRecord'
```

Shto atë CNAME te Cloudflare **me proxy të fikur** (renë gri): validimi i ACM-së nuk kalon
përmes proxy-t. Kur statusi bëhet `ISSUED`, vendos ARN-në te
`infra/terraform/envs/staging/terraform.tfvars`, te `acm_certificate_arn`.

## 3. Ngritja e infrastrukturës

```bash
cd infra/terraform/envs/staging && terraform init
```

```bash
terraform plan -out staging.tfplan
```

Lexo planin para se ta zbatosh. Prit rreth 60 burime: VPC me tri zona, NAT, ALB, Aurora,
Redis, SQS/SNS, S3, ECS dhe rolet e IAM-it.

```bash
terraform apply staging.tfplan
```

Aurora-ja merr 10–15 minuta. Kur mbaron, ruaj daljet:

```bash
terraform output
```

## 4. Vlerat e sekreteve

Terraform-i i krijon sekretet bosh; vlerat i vendos ti, sepse ai që i shkruan duhet të jetë
i vetmi që i sheh. Për secilin:

```bash
aws secretsmanager put-secret-value --secret-id krejt-staging/mapbox-token --secret-string '<VLERA>' --profile krejt-staging
```

| Sekreti | Përmbajtja |
| --- | --- |
| `krejt-staging/jwt` | JSON me `private_key_pem` dhe `otp_pepper` |
| `krejt-staging/google-maps` ose `krejt-staging/mapbox-token` | çelësi i ofruesit që zgjedh me `MAPS_PROVIDER` |
| `krejt-staging/infobip` | çelësi i API-së për SMS |
| `krejt-staging/payment-provider` | JSON me `secret_key` dhe `webhook_secret` |
| `krejt-staging/fcm` | JSON-i i llogarisë së shërbimit |
| `krejt-staging/centrifugo` | çelësi i API-së dhe sekreti HMAC |
| `krejt-staging/sentry`, `posthog`, `otel` | DSN dhe çelësat e observability-t |

Çelësin RSA për JWT gjeneroje lokalisht dhe mos e ruaj askund tjetër:

```bash
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out jwt-staging.pem
```

## 5. Imazhi i parë dhe ndezja e shërbimeve

Rrjedha `deploy-dev` në GitHub Actions bën push në ECR me OIDC. Për staging-un, shto te
cilësimet e repo-s variablën e mjedisit me ARN-në e rolit që e nxjerr:

```bash
terraform output deploy_role_arn
```

Pasi imazhi i parë të jetë në ECR, rrit numrin e detyrave te `terraform.tfvars`
(`api_desired_count = 1`, `worker_desired_count = 1`, `centrifugo_desired_count = 1`) dhe
zbato përsëri.

## 6. DNS-ja

Merr emrin e ALB-së:

```bash
terraform output alb_dns_name
```

Te Cloudflare shto një CNAME `staging` → ai emër, **me proxy të ndezur** (renë portokalli).
Modaliteti TLS te Cloudflare duhet **Full (strict)**, që lidhja të mbetet e enkriptuar edhe
mes Cloudflare-it dhe ALB-së.

## 7. Provo që punon

```bash
curl -fsS https://staging.krejt.app/healthz && echo
```

```bash
curl -fsS https://staging.krejt.app/api/v1/config | head -c 400 && echo
```

Migrimet ekzekutohen vetë kur niset API-ja, me `pg_advisory_lock`, ndaj dy detyra që nisen
njëkohësisht nuk e prishin njëra-tjetrën.

## 8. Aplikacionet kundrejt staging-ut

```bash
cd apps/customer && flutter build apk --debug --dart-define=KREJT_API_BASE_URL=https://staging.krejt.app
```

Të njëjtën gjë për `apps/driver`. Për panelet, vendos `KREJT_API_BASE_URL` te mjedisi ku
shërbehen.

## Kur diçka nuk shkon

**ALB-ja kthen 503** — asnjë detyrë e shëndetshme. Shih `aws ecs describe-services` dhe
log-un e detyrës te CloudWatch; shpesh është një sekret bosh që e ndal nisjen.

**API-ja nuk lidhet me bazën** — grupi i sigurisë i Aurora-s pranon vetëm nga grupi i
aplikacionit. Nëse e ke ndryshuar me dorë, `terraform apply` e kthen te gjendja e përshkruar.

**Certifikata mbetet `PENDING_VALIDATION`** — CNAME-ja e validimit është pas proxy-t të
Cloudflare-it. Fike renë për atë regjistrim.

**Fshirja e mjedisit** — `terraform destroy` punon te staging-u sepse `deletion_protection`
është e fikur. Te prodhimi është e ndezur me qëllim.
