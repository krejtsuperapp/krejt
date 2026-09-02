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

## Faza 2 (vazhdim) — `users` + outbox real

**Outbox → SNS (§41).** Modulet shkruajnë ngjarjet me `events.Emit(ctx, tx, …)` në të njëjtin transaksion me ndryshimin.
Worker-i (`internal/workers/outbox`) i lexon me `FOR UPDATE SKIP LOCKED` (disa instanca pa dyfishim), i publikon në
SNS `domain-events` (atribute `event_type`, `aggregate_type` për filtrim në abonimet SQS), i shënon `published_at`.
Dështimi: `attempts++`, `next_attempt_at = now() + 2^n s` (max 10 min), `last_error`; ngjarjet pasuese të të njëjtit
agregat presin (renditja ruhet). Pas 10 përpjekjesh logohet në nivel ERROR (alarm). `EVENTS_PUBLISHER=devlog` lejohet
vetëm në development dhe vetëm logon.

**Moduli `users`** (`/api/v1/users/me/*`, të gjitha pas `RequireAuth`):

| Metoda | Rruga | Çfarë bën |
|---|---|---|
| PATCH | `/users/me` | emri, emaili (unik), gjuha sq/en/de — `""` pastron emrin/emailin |
| DELETE | `/users/me` | fshirje e butë + anonimizim; kërkon wallet = 0 (`WALLET_NOT_EMPTY`) |
| GET/POST | `/users/me/addresses` | adresat e ruajtura (max 20); e para bëhet parazgjedhje |
| PUT/DELETE | `/users/me/addresses/{id}` | ndryshim / fshirje e butë |
| GET/PUT | `/users/me/notification-preferences` | 8 kategori × push/email/sms; `security.push` s'çaktivizohet |
| GET | `/users/me/sessions` | pajisjet e kyçura (`current: true` për këtë) |
| DELETE | `/users/me/sessions/{id}` | shkyç një pajisje |

Rregulla: adresat pranohen vetëm brenda kufijve të Kosovës (§1; kuti kufizuese në V1, poligon me modulin `maps`) —
`ADDRESS_OUTSIDE_KOSOVO`. Gabimet e validimit kthejnë `error.fields {fusha: arsyeja}` për shfaqje inline (§57).
Çdo veprim: rresht në `audit_log` (actor, IP, request_id) + ngjarje në outbox (`UserProfileUpdated`, `UserAddressAdded`, `UserDeleted`).

Mungon ende (e shënuar, jo e simuluar): ri-autentikim/MFA para fshirjes së llogarisë, eksporti i të dhënave, poligoni i saktë i kufirit.
