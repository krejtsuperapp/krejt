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

## Faza 1 (thelbi) — udhëtimet: harta, çmimi, shoferët, lokacioni, dispatch

**MapProvider (§46)** — `platform/providers/maps`: `Provider.Route(from, to)`; zbatimi Google Routes API
(`computeRoutes`, TRAFFIC_AWARE). `MAPS_PROVIDER=devestimate` (vetëm development) vlerëson vijë ajrore × 1.3 me 25 km/h.

**pricing** — zona (`service_areas`, Prishtina aktive), kategoritë (economy/comfort/xl/taxi), rregullat për zonë × kategori
(bazë, €/km, €/min, minimum, tarifë anulimi + periudhë hiri, surge në pikë bazë, komision). `POST /rides/quote` kthen një
ofertë (quote) për kategori, me afat 2 min; çmimi llogaritet vetëm me numra të plotë dhe rrumbullakohet lart në 10 cent.
Klienti dërgon `quote_id`, kurrë çmim.

**drivers** — `POST/GET /driver/profile` (aplikim me automjet + kategori), `PATCH /admin/drivers/{id}` (OPERATIONS: approve →
jep RIDE_DRIVER/TAXI_DRIVER; suspend → i heq), `GET /admin/drivers` (në pritje). `POST /driver/online|offline`,
`POST /driver/location` (grup mostrash: `{samples:[{lat,lng,ts,heading,speed_mps}]}`).

**location (§27)** — Redis GEO `geo:drivers:{kategori}` + hash `driver:{id}` (status, koha e fundit, ride_id), TTL 3 min;
mostrat renditen sipas kohës (dublikatat/jashtë-rendit hidhen), të ndenjurit (>60 s) nuk marrin oferta dhe pastrohen.
Në PostgreSQL ruhen vetëm mostra gjatë udhëtimit (~çdo 30 s) — kurrë çdo GPS update.

**rides (§18)** — makinë gjendjesh: `matching → assigned → arrived → in_progress → completed`; `cancelled`; `no_driver`;
shoferi që anulon e kthen udhëtimin në `matching` (ricaktim) dhe shënohet në audit. Një udhëtim aktiv për klient/shofer
(indeks unik në DB). `POST /rides` me header `Idempotency-Key`. Anulimi nga klienti: falas në kërkim dhe brenda periudhës
së hirit, pastaj tarifa e rregullit (arkëtohet nga wallet-i; për cash regjistrohet, nuk arkëtohet në V1).
Pagesa në përfundim (settle, idempotente): wallet → debit klienti, kredit wallet-i i shoferit (çmimi − komisioni), kredit
`krejt:commission`; cash → shoferi mban cash-in, komisioni i debitohet wallet-it të tij (borxh). `card` refuzohet hapur
(`PAYMENT_METHOD_UNAVAILABLE`) derisa të vijë moduli `payments`.

**dispatch (§26, v1)** — worker-i çdo sekondë: skadon ofertat (TTL 20 s), për çdo udhëtim në kërkim pa ofertë të hapur
zgjedh kandidatin më të afërt (Redis GEO, 6 km) që është i miratuar, ka kategorinë, s'ka udhëtim aktiv dhe s'e ka marrë
ofertën më parë; pas 3 min → `no_driver`. Shoferi i sheh ofertat te `GET /driver/offers` (deri te Centrifugo: polling 3 s
nga aplikacioni i shoferit) dhe përgjigjet me `/accept` ose `/decline`.

Ende jo (e shënuar): push/Centrifugo për ofertat dhe gjurmimin, vlerësimet, chat-i, SOS, dokumentet e shoferit me skadim,
surge dinamik, poligonet H3 të zonave, rillogaritje e çmimit kur rruga ndryshon shumë.

## Njoftimet (§29, §47)

Rrjedha: modulet shkruajnë ngjarje në outbox → worker-i i publikon në SNS `domain-events` → SQS `notifications` →
konsumatori (`internal/workers/queue`) thërret `notifications.Handle(ngjarja)`. Në development pa AWS
(`EVENTS_PUBLISHER=devlog`) e njëjta ngjarje i jepet `Handle` direkt në proces — të njëjtat funksione, pa simulim.

`Handle`: harton ngjarjen te marrësit (`Map`: kush, kategori, tekst, deep link, prioritet/TTL), shkruan rreshtin në kutinë
e aplikacionit (`notifications`, unik për ngjarje+përdorues → ridorëzimi i SQS nuk dyfishon asgjë), zbaton preferencat
(§29; `security` gjithmonë push), dërgon push në çdo pajisje të vlefshme në **gjuhën e pajisjes** (sq/en/de) dhe shkruan
gjurmën e dorëzimit (`notification_deliveries`: sent/failed/skipped, token i shkurtuar). Token-at që FCM i kthen
UNREGISTERED shënohen `invalid_at` dhe nuk përdoren më.

`PushProvider` (§47): FCM HTTP v1 me llogari shërbimi (OAuth2 JWT RS256, token i ruajtur deri në skadim), prioritet/TTL/
collapse për Android dhe APNs. `PUSH_PROVIDER=devlog` vetëm në development.

API: `POST/DELETE /api/v1/notifications/push-token` (regjistrim/rifreskim/çregjistrim), `GET /api/v1/notifications`
(kutia + `unread`), `POST /api/v1/notifications/{id}/read`, `POST /api/v1/notifications/read-all`.

Ende jo: email (Postmark) dhe SMS jo-OTP si kanale njoftimi, njoftimet e planifikuara/marketing, Centrifugo për ekranet live.

## Kanalet e gjalla (§42) — Centrifugo

`RealtimeProvider`: Centrifugo (HTTP API `publish`, çelës API) + `TokenIssuer` (JWT HS256 me të njëjtin sekret si
`token_hmac_secret_key` i Centrifugo-s). API: `POST /api/v1/realtime/token` (token lidhjeje, 1 h) dhe
`POST /api/v1/realtime/subscribe {channel}` → token abonimi (2 h) **vetëm pas autorizimit server-side**:
`ride:{id}` për klientin/shoferin e udhëtimit, `driver:{id}` dhe `user:{id}` vetëm për vetveten.

Publikimet: worker-i (i njëjti përpunues si njoftimet) → `ride:{id}` për RideAssigned/DriverArrived/Started/Completed/
Cancelled/NoDriver/Payment*, `driver:{id}` për RideOffered/RideOfferExpired (dhe anulimin nga klienti); moduli `location`
publikon pozicionin e shoferit në `ride:{id}` në çdo mostër të pranuar gjatë udhëtimit. Kështu klienti i shoferit nuk ka
më nevojë për polling të ofertave (mbetet si rezervë), dhe klienti e sheh makinën të lëvizë pa pyetur serverin.

Në AWS: Centrifugo si shërbim ECS (Redis engine), i arritshëm nga api/worker përmes Cloud Map
(`centrifugo.<name>.local:8000`); klientët lidhen përmes ALB `/connection/*`. Lokalisht: `docker compose up` ngre
Centrifugo-n me sekrete vetëm-për-laptop.

## Vlerësimet (§30) dhe dokumentet e shoferit (§18, §51)

**reviews** — `POST /rides/{id}/review` (rolin e nxjerr serveri: klienti → shoferi, shoferi → klienti), vetëm për udhëtime
të përfunduara, brenda 7 ditësh, një herë për person (unik në DB); etiketa të lejuara për rol, koment ≤ 300; agregati
(shuma/numërimi) te `drivers`/`users` përditësohet në të njëjtin transaksion. `POST /reviews/{id}/report` → `flagged`
(pala e vlerësuar e raporton; Support e moderon). `GET /rides/{id}/reviews` kthen të vetat + të tjetrit vetëm nëse `visible`.

**documents** — ngarkim direkt në S3 me URL të nënshkruar (`POST /driver/documents/upload-url` → PUT; `STORAGE_PROVIDER=devfs`
vetëm në development me `PUT /api/v1/dev/uploads/{key}`), pastaj `POST /driver/documents` që verifikon objektin (ekziston,
lloji dhe madhësia përputhen me të premtuarën — jpeg/png/pdf, ≤ 10 MB) dhe datën e skadimit. Llojet: foto profili, ID,
patentë, librezë, sigurim, dëshmi e pastërtisë (+ certifikatë taksie për kategorinë taxi). Operacionet: `GET /admin/driver-documents`,
`GET /admin/drivers/{id}/documents` (URL leximi 5 min), `PATCH /admin/driver-documents/{id}` approve/reject.
**Miratimi i shoferit tani kërkon dokumentet e detyrueshme të miratuara** (`DRIVER_DOCUMENTS_INCOMPLETE` me listën).
Worker-i (çdo orë) skadon dokumentet e miratuara me datë të kaluar dhe **pezullon** shoferin (kapacitetet hiqen, ngjarje →
njoftim) derisa ta rinovojë.

Ende jo: skanim malware i skedarëve (§51), OCR/lexim automatik, rikthim automatik i shoferit pas rinovimit (bëhet nga ops).

## Pagesat me kartë dhe mbushja e wallet-it (§5, §24)

`PaymentProvider` — Stripe (entiteti në BE): `payment_intents` me `automatic_payment_methods`, çelës idempotence,
rimbursime, webhook me nënshkrim (`Stripe-Signature`, HMAC-SHA256, tolerancë 5 min). Raiffeisen (gateway lokal) vjen pas
së njëjtës ndërfaqe kur të ketë kontratë. `PAYMENT_PROVIDER=devlog` vetëm në development: qëllimet krijohen, por **asnjë
pagesë nuk kalon vetvetiu** — suksesi/dështimi simulohet me `POST /api/v1/dev/payments/{id}/succeed|fail` (webhook i
nënshkruar me sekretin dev).

Rrjedha: `POST /wallet/topup` (Idempotency-Key, 1,00–500,00 €, shumëfish i 0,50 €, ≤ 1.000 € / 24 h) → rresht `created`
→ ofruesi → `client_secret` te aplikacioni → klienti konfirmon me SDK-në e ofruesit → **webhook** (`POST
/payments/webhook/stripe`): nënshkrimi verifikohet, ngjarja regjistrohet një herë (`payment_webhook_events`), shuma
krahasohet me tonën (mospërputhje → `failed/amount_mismatch`, pa kreditim), kreditimi në ledger me çelës
`topup:{intent}` (debit `krejt:card_clearing`, kredit wallet-i), ngjarje `WalletToppedUp` → njoftim. `GET
/payments/intents/{id}` për statusin. Finance: `POST /admin/payments/intents/{id}/refund` (pjesërisht/plotësisht; wallet-i
duhet të ketë mjaftueshëm — paratë e shpenzuara nuk kthehen dot).

Wallet (§5): `GET /wallet` (bilanci nga ledger-i, `closed_loop: true`, limitet), `GET /wallet/transactions` (hyrjet e
ledger-it të përdoruesit: mbushje, udhëtime, tarifa anulimi, rimbursime). Asnjë transfer mes përdoruesve, asnjë tërheqje.
Tarifa e kartës absorbohet nga KREJT në V1. Ende jo: rakordim automatik me raportet e ofruesit, 3-D Secure si kërkesë e
detyruar (varet nga ofruesi/karta), Raiffeisen.

## Konfigurimi i platformës (§64, §65) dhe kufizimi i shpejtësisë (§51)

`GET /api/v1/config` (publik, cache 30 s; me token opsional për flag-et e personalizuara): versioni minimal/i
rekomanduar për `X-App-Id`/`X-App-Platform`/`X-App-Version`, `update_state` (ok | recommended | required |
maintenance), flag-et publike të vlerësuara për përdoruesin. Porta e versionit (middleware) kthen `UPDATE_REQUIRED`
(426) nën versionin minimal dhe `MAINTENANCE` (503) kur mirëmbajtja është aktive — për të gjitha rrugët përveç
shëndetit, `/config` dhe webhook-eve. Admin (OPERATIONS/SUPER_ADMIN): `GET/PATCH /admin/flags/{key}`
(enabled, rollout_percent, public), `GET /admin/app-versions`, `PUT /admin/app-versions/{app}/{platform}` — çdo ndryshim
në audit dhe i gjallë brenda 30 s në të gjitha instancat.

Flag-et janë të deklaruara (tabela `feature_flags`), jo të shpërndara në kod: `rides.request` (kërkesa e udhëtimit),
`wallet.topup` (mbushja), `rides.surge_dynamic` (i mbyllur), `drivers.self_signup`, `notifications.push`. Rollout-i
për përqindje është determinist për përdorues (hash i çelësit + id-së).

Kufizimi i shpejtësisë: 300 kërkesa/min për IP (para autentikimit) dhe 600/min për përdorues (pas RequireAuth), dritare
fikse në Redis me `Retry-After`; OTP-ja mban kufijtë e vet më të rreptë. Nëse Redis-i nuk përgjigjet, kërkesa kalon
(fail-open) dhe logohet — kufizimi nuk rrëzon shërbimin.

## Mbështetja (§36) dhe Admin (§35)

**support** — tiketa (`POST/GET /support/tickets`, `GET /support/tickets/{id}`, `POST …/messages`, `POST …/close`) me kategori
(ride/order/payment/refund/account/safety/other) dhe prioritet automatik (safety → urgent, payment/refund → high);
udhëtimi i lidhur verifikohet si i përdoruesit. `POST /safety/reports` (SOS, ngasje e rrezikshme, ngacmim, aksident…) →
tiketë urgjente + `SafetyReportCreated` (kanali live `ops:` për panelin e Operacioneve). Agjentët (SUPPORT/ADMIN):
`GET /admin/support/tickets?status=&priority=&assigned=me` (urgentët para), `GET /admin/support/tickets/{id}` (mesazhet +
konteksti: telefon/emër/gjuhë, përmbledhja e udhëtimit — qasja auditohet), `POST …/messages` (→ `pending_user`, njoftim
push te përdoruesi), `PATCH …` (status/prioritet/caktim; mbyllja mbyll edhe raportin e sigurisë).

**admin** (lexim; ADMIN/SUPPORT/OPERATIONS/FINANCE, SUPER_ADMIN gjithmonë) — `GET /admin/users?q=` (telefon/email/emër/id),
`GET /admin/users/{id}` (kapacitetet, wallet-i, udhëtimet, sesionet, profili i shoferit, audit-i i fundit; qasja auditohet),
`GET /admin/rides?state=&customer_id=&driver_id=`, `GET /admin/rides/{id}` (ngjarjet e makinës së gjendjeve + ofertat e
dispatch-it), `GET /admin/dispatch/live` (udhëtimet aktive, ofertat e hapura, shoferët online sipas kategorisë nga Redis,
SOS të hapura), `GET /admin/audit` (vetëm ADMIN). Veprimet me peshë mbeten në modulet e tyre (drivers, documents,
support, payments, appconfig) — çdonjëra me audit.

Ende jo: chat klient↔shofer (§28), moderimi i vlerësimeve nga Support si endpoint, analitika (§66), fraud v1 (§67) si modul.

## Chat klient ↔ shofer (§28)

`POST /rides/{id}/chat` dhe `GET /rides/{id}/chat?after=` (leximi shënon mesazhet e palës tjetër si të lexuara) — vetëm
mes palëve të udhëtimit, nga caktimi i shoferit deri 24 h pas përfundimit (1 h pas anulimit). Çdo mesazh emeton
`RideChatMessage`: kanali live `ride:{id}` e dorëzon menjëherë; marrësi merr push me parapamje (grumbulluar për udhëtim).
Raportimi: tiketë mbështetjeje me `ride_id`; Support-i e sheh udhëtimin, jo chat-in e plotë pa kërkesë (moderimi vjen me
panelin). Ruajtja: mesazhet fshihen pas 90 ditësh (worker `chat.retention`). Worker-i i mirëmbajtjes tani mban punë me
intervale të veçanta (`documents.expire` çdo orë, `chat.retention` çdo 6 orë), secila edhe në nisje.

## Observability (§50) dhe analitika (§66)

**OpenTelemetry**: span për çdo kërkesë HTTP (emri = shablloni i rrugës), për çdo query të PostgreSQL (pgx tracer,
vetëm teksti i SQL-së pa argumente) dhe çdo komandë Redis; gjurmë + metrika (HTTP durations) drejt OTLP
(`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS` nga Secrets Manager `krejt/<env>/otel`,
`OTEL_TRACES_SAMPLER_ARG` = përqindja). `trace_id` hyn në log dhe në zarfin e gabimit (`error.trace_id`) — klienti mund ta
tregojë te Support-i. Pa endpoint (development) asgjë nuk eksportohet.

**Sentry** (`SENTRY_DSN`): paniqet dhe gabimet e brendshme 5xx me shkak, me `request_id`/`trace_id`/`user_id` (vetëm id,
`SendDefaultPII=false`); gjurmët mbeten te OpenTelemetry.

**Analitika** (PostHog EU, `POSTHOG_KEY`): worker-i i kthen ngjarjet e outbox-it në ngjarje produkti — signup,
ride_requested/accepted/completed/cancelled/no_driver, payment_success/failure, wallet_topup, review, support_ticket,
safety_report, driver_applied/approved — me `distinct_id` = id-ja e përdoruesit dhe veti vetëm biznesi (asnjë telefon/
email/emër). Dërgim në grup (5 s / 50 ngjarje), `devlog` vetëm në development.

Terraform: sekreti i ri `otel` (header-i i autorizimit të Grafana Cloud), variabla `otlp_endpoint`, `SENTRY_DSN` dhe
`POSTHOG_KEY` në task-et ECS. Rregulli i §50: kurrë fjalëkalime/token-a/çelësa/sekrete pagese në log (maskimi i logx).
