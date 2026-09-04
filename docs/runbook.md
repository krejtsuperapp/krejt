# Runbook — operimi i KREJT (§44, §47, §50, §71)

## Shëndeti
- `GET /healthz` (procesi), `GET /readyz` (DB + Redis). ALB-ja shëndetëson `/healthz`; Centrifugo `/health`.
- Log-e JSON me `request_id`/`trace_id`/`user_id` (CloudWatch → Grafana Cloud); gjurmë/metrika OTLP; gabime në Sentry.
- Alarmet e para (CloudWatch, moduli messaging): DLQ jo bosh për çdo radhë; buxheti 60 USD (80 % real / 100 % parashikim).
- Në log, `outbox publish failed` me `attempt ≥ 10` = SNS i paarritshëm ose ngjarje e prishur — shiko `outbox_events.last_error`.

## Rikthimi (§71)
- Aurora: backup automatik + PITR (`backup_retention_period` në modulin data). Rikthim: `restore-db-cluster-to-point-in-time`
  → cluster i ri → ndërro `DB_WRITER_HOST` (Terraform var) → deploy.
- **Ushtrimi i rikthimit** (`infra/terraform/drills/aurora-restore`): ngre një cluster të përkohshëm nga kopja
  e momentit të fundit, me Data API, dhe verifikon me SQL pa hyrë në VPC:
  `terraform init && terraform plan -out drill.tfplan -var source_cluster=krejt-dev-aurora && terraform apply drill.tfplan`,
  pastaj `aws rds-data execute-statement --resource-arn <cluster_arn> --secret-arn <secret_arn> --database krejt --sql "select count(*) from users"`,
  në fund `terraform plan -destroy -out d.tfplan … && terraform apply d.tfplan`. Nëse Data API del e fikur pas
  rikthimit (ndodh: rikthimi e injoron), një `plan`/`apply` i dytë e ndez.
- Ushtrimi i fundit: **04.09.2026**, dev — cluster i gatshëm pas 7 min, kopja e plotë (5 përdorues, 25 udhëtime,
  12 porosi, 59 regjistrime ledger-i, udhëtimi i fundit 22:31 UTC i një nate më parë), pastaj u shkatërrua.
  Përsërite çdo tremujor dhe pas çdo ndryshimi të modulit `data`.
- Redis: gjendje kalimtare (GEO, kufij, sesione live) — humbja e tij nuk humb para; shoferët dalin online sërish.
- S3: versionim aktiv; fshirja e gabuar kthehet nga versioni i mëparshëm.
- Sekretet: Secrets Manager me KMS; rotacion manual (JWT: gjenero çelës të ri, vendos, ri-nis; sesionet mbeten se
  refresh-i ri-lëshon access token me çelësin e ri).

## Incidente të zakonshme
| Simptoma | Kontroll | Veprim |
|---|---|---|
| Klientët marrin 503 `MAINTENANCE` | `app_versions.maintenance` | `PUT /admin/app-versions/{app}/{platform}` `{maintenance:false}` |
| Asnjë ofertë për shoferët | `GET /admin/dispatch/live` (online_drivers), log-et e worker-it `dispatch sweep` | Redis i arritshëm? worker-i po ecën? shoferët online me kategorinë e duhur? |
| Njoftimet nuk arrijnë | `notification_deliveries.status`, DLQ `notifications` | token-a të pavlefshëm (FCM) ose SQS i pakonsumuar (worker) |
| Pagesa "u pagua" te klienti por wallet-i s'u rrit | `payment_webhook_events` për `event_id`, `payment_intents.status` | webhook-u nuk mbërriti (Stripe → riprovo) ose `amount_mismatch` (Finance) |
| Shoferi nuk merr payout | `driver_bank_accounts`, bilanci ≥ 5 €, statusi approved | grupi tjetër javor; `PATCH /admin/payouts/items/{id}` për dështime |
| Rritje e papritur e kërkesave | `risk_flags`, rate-limit 429 në log | `POST /admin/users/{id}/block` me arsye; flag `rides.request` off për ndalim emergjent |

## Ndalime emergjente (§65)
`PATCH /admin/flags/rides.request` `{enabled:false}` (kërkesat e reja), `wallet.topup` (mbushjet), `notifications.push`.
Ndryshimet janë live brenda 30 s në të gjitha instancat.

## Deploy / rollback
Shih `docs/deploy.md`. Rollback: `aws ecs update-service --task-definition <revizioni i mëparshëm>` (circuit breaker-i e bën
vetë kur task-et e reja dështojnë në nisje).
