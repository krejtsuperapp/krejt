# Deploy (§70, §72)

## Rrjedha
1. PR → CI: gofmt, vet, staticcheck, teste (njësi + integrimi me Postgres/Redis), govulncheck, build, imazh ARM64 + Trivy,
   gitleaks, terraform fmt/validate. Asnjë bashkim pa këto porta (ruleset i `main`).
2. Bashkim në `main` → punë `deploy-dev`: hyrje në AWS me **OIDC** (roli `krejt-dev-github-deploy` nga Terraform, moduli
   `cicd`; lejon vetëm ECR push, regjistrim task definition-i dhe `UpdateService` për shërbimet `krejt-dev-*`), build + push
   `api`/`worker` me tag = SHA, revizion i ri task definition-i, `update-service`, `wait services-stable`.
   Circuit breaker-i i ECS bën rollback automatik nëse task-et e reja dështojnë.
3. Staging/prod: e njëjta rrjedhë me mjedise të veçanta GitHub (`staging`, `prod` me miratim manual) dhe role OIDC për
   llogaritë përkatëse — shtohen kur të krijohen mjediset në Terraform.

## Njëherësh (nga pronari i llogarisë AWS, jo nga CI)
- `terraform apply` në `envs/dev` (krijon rolin OIDC) → `terraform output deploy_role_arn`.
- GitHub → Settings → Environments → `dev` → Secret `AWS_DEPLOY_ROLE_ARN` = vlera e mësipërme. Pa këtë sekret,
  puna `deploy-dev` kapërcehet me mesazh të qartë (nuk dështon).
- Vlerat e sekreteve në Secrets Manager (`krejt-dev/jwt` JSON {private_key_pem, otp_pepper}, `google-maps`, `infobip`,
  `fcm`, `centrifugo` JSON {api_key, token_hmac_secret_key}, `payment-provider` JSON {secret_key, webhook_secret},
  `sentry`, `posthog`, `otel`) — task-et nuk nisen pa to.
- Pas imazhit të parë: `alb_enabled = true`, `nat_enabled = true`, `api_desired_count`/`worker_desired_count` ≥ 1 në
  `envs/dev`, `terraform apply`. Terraform nuk e prek më `task_definition`-in e shërbimeve (deploy-i e menaxhon).

## Lokalisht
`docker compose up --build` (Postgres, Redis, Centrifugo, api, worker me ofrues dev). `make test` për testet e integrimit.

## Testi i ngarkesës (§69)
`tests/load/rides.js` (k6): oferta çmimi me rritje deri 90/s, mostra GPS 200/s, rrjedha kërkesë→anulim; pragjet p95
quote < 500 ms, request < 800 ms, gabime < 1 %. Ekzekutohet kundër **staging-ut** me token-a të llogarive të testit:
`k6 run -e BASE=https://api-staging.krejt.app -e CUSTOMER_TOKEN=… -e DRIVER_TOKEN=… tests/load/rides.js`.
Kriteri i nisjes (Faza 1): 30 ditë pa incident P1 në staging nën 2–3× e pikut.

## Prodhimi (api.krejt.app)

1. Certifikata: `terraform apply -target=aws_acm_certificate.api` te `infra/terraform/envs/prod` (profil `krejt-prod`),
   pastaj `terraform output acm_validation_record` → CNAME te Cloudflare **me proxy të fikur**. Prit `ISSUED`.
2. Ngritja me faza (mësim nga staging-u): `-target=module.network` → `-target=module.ecs` → apply i plotë.
   Sekretet e prod-it (JWT, OTP, Centrifugo, Mapbox, FCM, Infobip, Stripe, PostHog) vendosen te Secrets Manager
   **nga makina e operatorit**; në prod serveri refuzon `devlog` për SMS/pagesa/analitikë.
3. DNS: CNAME `api` → ALB-ja e prod-it (proxy i ndezur). Panelet me emra një-nivelësh (`admin.krejt.app`, `partner.krejt.app`).
4. GitHub: mjedisi `prod` me **required reviewers** (miratimi manual), sekreti `AWS_DEPLOY_ROLE_ARN` = output-i
   `deploy_role_arn`, variabla e repo-s `PROD_DEPLOY=true`. Vetëm atëherë `deploy-prod` ekzekutohet — pas
   `smoke-dev` dhe `health-staging`, me të njëjtën SHA. `health-prod` verifikon `/healthz` dhe versionin e shërbyer.
