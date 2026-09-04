-- Pakot (§21, Faza 2): dërgesa brenda qytetit nga klienti te një marrës, me korrier.
-- E njëjta rrjedhë si porositë: ofertë çmimi (2 min) → kërkesë → oferta korrierëve → marrje me kod →
-- dorëzim me kod → shlyerje në ledger. Kodet: pickup_code (dërguesi ia thotë korrierit),
-- delivery_code (marrësi ia thotë korrierit) — klienti i sheh të dyja te aplikacioni.

CREATE TABLE parcel_pricing (
  size            text PRIMARY KEY CHECK (size IN ('s','m','l')),
  base_minor      bigint NOT NULL CHECK (base_minor >= 0),
  per_km_minor    bigint NOT NULL CHECK (per_km_minor >= 0),
  commission_bp   int NOT NULL DEFAULT 2000 CHECK (commission_bp BETWEEN 0 AND 10000),
  currency        text NOT NULL DEFAULT 'EUR',
  updated_at      timestamptz NOT NULL DEFAULT now()
);
-- Vlera fillestare për dev/staging; Operacionet i ndryshojnë para hapjes publike.
INSERT INTO parcel_pricing (size, base_minor, per_km_minor) VALUES
  ('s', 150, 40), ('m', 200, 60), ('l', 300, 90);

CREATE TABLE parcel_quotes (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id      uuid NOT NULL REFERENCES users(id),
  size             text NOT NULL REFERENCES parcel_pricing(size),
  pickup_lat       double precision NOT NULL,
  pickup_lng       double precision NOT NULL,
  pickup_address   text,
  dropoff_lat      double precision NOT NULL,
  dropoff_lng      double precision NOT NULL,
  dropoff_address  text,
  distance_m       int NOT NULL,
  duration_s       int NOT NULL,
  price_minor      bigint NOT NULL,
  commission_bp    int NOT NULL,
  currency         text NOT NULL,
  expires_at       timestamptz NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX parcel_quotes_customer_idx ON parcel_quotes(customer_id, created_at DESC);

CREATE TABLE parcels (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                  text NOT NULL UNIQUE,      -- referenca e shkurtër (K7F3QA)
  pickup_code           text NOT NULL,             -- 4 shifra: dërguesi → korrieri
  delivery_code         text NOT NULL,             -- 4 shifra: marrësi → korrieri
  customer_id           uuid NOT NULL REFERENCES users(id),
  courier_id            uuid REFERENCES drivers(user_id),
  quote_id              uuid REFERENCES parcel_quotes(id),
  state                 text NOT NULL CHECK (state IN ('requested','courier_assigned','picked_up','delivered','cancelled','no_courier')),
  size                  text NOT NULL REFERENCES parcel_pricing(size),
  payment_method        text NOT NULL CHECK (payment_method IN ('cash','wallet')),
  payment_status        text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending','paid','failed','cash','none')),
  pickup_lat            double precision NOT NULL,
  pickup_lng            double precision NOT NULL,
  pickup_address        text,
  pickup_contact_name   text,
  pickup_contact_phone  text,
  dropoff_lat           double precision NOT NULL,
  dropoff_lng           double precision NOT NULL,
  dropoff_address       text,
  recipient_name        text NOT NULL,
  recipient_phone       text NOT NULL,
  note                  text,
  distance_m            int NOT NULL,
  duration_s            int NOT NULL,
  price_minor           bigint NOT NULL CHECK (price_minor >= 0),
  commission_minor      bigint NOT NULL DEFAULT 0,
  currency              text NOT NULL,
  cancelled_by          text CHECK (cancelled_by IN ('customer','courier','ops','system')),
  cancellation_reason   text,
  idempotency_key       text NOT NULL,
  created_at            timestamptz NOT NULL DEFAULT now(),
  assigned_at           timestamptz,
  picked_up_at          timestamptz,
  delivered_at          timestamptz,
  cancelled_at          timestamptz,
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (customer_id, idempotency_key)
);
CREATE INDEX parcels_customer_idx ON parcels(customer_id, created_at DESC);
CREATE INDEX parcels_open_idx ON parcels(created_at) WHERE state = 'requested';
-- Një pako aktive për korrier — si te porositë.
CREATE UNIQUE INDEX parcels_one_active_per_courier ON parcels(courier_id) WHERE state IN ('courier_assigned','picked_up');

CREATE TABLE parcel_events (
  id          bigserial PRIMARY KEY,
  parcel_id   uuid NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
  from_state  text,
  to_state    text NOT NULL,
  actor_type  text NOT NULL CHECK (actor_type IN ('customer','courier','ops','system')),
  actor_id    uuid,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX parcel_events_parcel_idx ON parcel_events(parcel_id, created_at);

CREATE TABLE parcel_offers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parcel_id    uuid NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
  courier_id   uuid NOT NULL REFERENCES drivers(user_id),
  round        int NOT NULL,
  state        text NOT NULL DEFAULT 'offered' CHECK (state IN ('offered','accepted','declined','expired','withdrawn')),
  distance_m   int NOT NULL,
  eta_s        int NOT NULL,
  expires_at   timestamptz NOT NULL,
  responded_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (parcel_id, courier_id)
);
CREATE INDEX parcel_offers_open_courier_idx ON parcel_offers(courier_id, expires_at) WHERE state = 'offered';
CREATE INDEX parcel_offers_open_expiry_idx ON parcel_offers(expires_at) WHERE state = 'offered';
