# KREJT backend — Faza 0

Monolit modular në Go (§39). Dy binarë nga i njëjti kod: `api` (HTTP, pas ALB) dhe `worker` (SQS).

## Struktura

```
backend/
  cmd/api            hyrja HTTP: /healthz, /readyz, /api/v1/*
  cmd/worker         konsumatorët e SQS (outbox → SNS, dispatch, notifications, payouts)
  internal/
    domain/money     paraja si numër i plotë në cent (§5, §23) — asnjë float
    modules/ledger   regjistri me dy hyrje: Post (idempotent, atomik), Balance, EnsureAccount
    platform/config  konfigurimi nga mjedisi (§73); kredencialet nga Secrets Manager
    platform/db      pgx pool + migrime SQL të embed-uara (schema_migrations)
    platform/cache   Redis (ElastiCache cluster / lokal), TLS + auth
    platform/httpx   middleware (request id, recover, secure headers, timeout, access log), gabimet (§57), health
    platform/logx    log JSON me request_id/trace_id/user_id; fushat sekrete maskohen (§50)
```

## Rregullat që zbaton kodi

- **Serveri është autoritar** (§51/§53): asnjë çmim, bilanc, rol apo status nuk pranohet nga klienti.
- **Ledger** (§23): `SUM(debit) = SUM(credit)` kontrollohet edhe në Go (`Validate`) edhe në PostgreSQL (trigger i shtyrë në COMMIT); rreshtat janë të pandryshueshëm (trigger që ndalon UPDATE/DELETE); `idempotency_key` unik → riprovimi kthen të njëjtin transaksion.
- **Gabimet** (§57): një format i vetëm `{error:{code,message_key,http_status,request_id,trace_id,retryable}}`; stack trace vetëm në log.
- **Log-et** (§50): JSON; `password/token/secret/otp/card/…` maskohen automatikisht.
- **Migrimet** (§40): `internal/platform/db/migrations/NNNN_*.sql`, të zbatuara në transaksion, një herë, me radhë.

## Lokalisht

```
docker compose up --build -d      # Postgres 16 + Redis 7 + api + worker
curl localhost:8080/readyz
make test                         # teste njësie + integrimi (ledger kundër Postgres-it lokal)
```

## Në AWS (dev)

Imazhi ndërtohet për **ARM64** (Fargate Graviton) dhe shtyhet në ECR `krejt-dev/api` dhe `krejt-dev/worker`.
Kredencialet e Aurora-s vijnë si `DB_CREDENTIALS_JSON` nga Secrets Manager (task definition), Redis-i si `REDIS_AUTH`.
Pas imazhit të parë: `nat_enabled = true`, `alb_enabled = true`, `api_desired_count = 2` në `envs/dev`.

## Çka vjen në Fazën 2 (identiteti)

`modules/identity` (users, capabilities), `modules/auth` (OTP përmes `SmsProvider` → Infobip, JWT RS256 jetëshkurtër + refresh me rotacion, sesione/pajisje, MFA), `modules/audit`, middleware i autorizimit me kapacitete.
