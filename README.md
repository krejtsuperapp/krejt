# KREJT — Super App (Kosovë)

Repo-ja e produktit. Burimi i së vërtetës: *KREJT — Final Master Prompt* (§0–§87). Vendimet e fiksuara: **vetëm Kosovë**,
**sq/en/de**, **wallet i mbyllur pa P2P** (cash + kartë), **AWS Frankfurt** (Aurora, ECS Fargate ARM64, ElastiCache, SQS/SNS, S3),
Google Maps, FCM, Infobip, Stripe (entiteti BE) + Raiffeisen, Centrifugo, Sentry/Grafana/PostHog. Serveri është autoritar;
paraja gjithmonë numër i plotë në cent; asnjë funksionalitet i simuluar (ofruesit `devlog` vetëm në development).

```
backend/     Go — monolit modular: cmd/{api,worker}, internal/{modules,platform,workers}, migrime SQL të embed-uara
infra/       Terraform (envs/{dev,staging,prod}, modules/{network,security,data,messaging,storage,ecs,cicd})
docs/        backend.md (arkitektura dhe çdo modul), deploy.md, dev-live.md, staging.md, runbook.md
```

## Ç'është ndërtuar (backend)
auth (OTP, JWT me rotacion, sesione) · users (profil, adresa vetëm-Kosovë, preferenca) · ledger me dy hyrje · wallet i mbyllur ·
payments (Stripe, webhook i nënshkruar, rimbursime) · rides (çmim server-side, makinë gjendjesh, anulim me tarifë, ricaktim) ·
drivers + documents (S3, shqyrtim, skadim) · location (Redis GEO) · dispatch v1 · chat · reviews · notifications (FCM, kuti,
preferenca) · realtime (Centrifugo) · support (tiketa, SOS) · admin (lexime, audit) · appconfig (versione, feature flags) ·
ratelimit · fraud v1 · payouts (IBAN, grupe javore, CSV) · observability (OpenTelemetry, Sentry) · analytics (PostHog) ·
OpenAPI i plotë me test mbulimi.

## Lokalisht
```
docker compose up --build        # Postgres 16, Redis 7, Centrifugo, api :8080, worker (ofrues dev)
curl localhost:8080/readyz
curl localhost:8080/api/v1/openapi.yaml
make test                        # teste njësie + integrimi (TEST_DATABASE_URL / TEST_REDIS_ADDR)
```

## AWS
`infra/terraform/envs/dev` — `terraform init && terraform plan && terraform apply` me profilin SSO `krejt-dev`. Deploy-i në dev
bëhet nga CI pas bashkimit në `main` (OIDC, pa çelësa) — shih `docs/deploy.md`.

## Rregulla
- Asnjë sekret në repo (`.env.example` mban vetëm emrat); asnjë burim AWS me dorë — vetëm Terraform.
- Çdo ndryshim përmes PR: gofmt, vet, staticcheck, teste, govulncheck, Trivy, gitleaks, terraform validate.
- Çdo endpoint i ri hyn në `openapi.yaml` (testi e detyron), çdo veprim me peshë në `audit_log`, çdo ngjarje përmes outbox-it.
