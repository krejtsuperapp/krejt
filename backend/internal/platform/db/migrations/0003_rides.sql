-- =============================================================================
-- KREJT — 0003: udhëtimet (§18), çmimi server-side, dispatch (§26), lokacioni (§27), shoferët.
-- Faza 1: Prishtina. Çmimi është i paracaktuar (upfront): klienti dërgon quote_id, kurrë çmim.
-- =============================================================================

-- ------------------------------ zonat e shërbimit -----------------------------
CREATE TABLE service_areas (
  id          text PRIMARY KEY,               -- 'prishtina'
  name        text NOT NULL,
  center_lat  double precision NOT NULL,
  center_lng  double precision NOT NULL,
  radius_km   double precision NOT NULL,      -- rreth; poligoni H3 vjen me modulin maps
  active      boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now()
);
INSERT INTO service_areas (id, name, center_lat, center_lng, radius_km, active) VALUES
  ('prishtina', 'Prishtinë', 42.6629, 21.1655, 15, true),
  ('prizren',   'Prizren',   42.2139, 20.7397, 10, false),
  ('peja',      'Pejë',      42.6600, 20.2883, 10, false),
  ('gjakova',   'Gjakovë',   42.3800, 20.4300, 10, false),
  ('mitrovica', 'Mitrovicë', 42.8914, 20.8660, 10, false),
  ('ferizaj',   'Ferizaj',   42.3700, 21.1550, 10, false),
  ('gjilan',    'Gjilan',    42.4635, 21.4694, 10, false);

CREATE TABLE ride_categories (
  id        text PRIMARY KEY,                 -- economy | comfort | xl | taxi
  name_key  text NOT NULL,                    -- çelës përkthimi (sq/en/de në klient)
  seats     smallint NOT NULL,
  sort      smallint NOT NULL,
  active    boolean NOT NULL DEFAULT true
);
INSERT INTO ride_categories (id, name_key, seats, sort) VALUES
  ('economy', 'ride.economy', 4, 1),
  ('comfort', 'ride.comfort', 4, 2),
  ('xl',      'ride.xl',      6, 3),
  ('taxi',    'ride.taxi',    4, 4);

-- ------------------------------ rregullat e çmimit ----------------------------
-- Para si numra të plotë në cent. surge/commission në pikë bazë (10000 = 1.00×).
CREATE TABLE pricing_rules (
  id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  service_area_id           text NOT NULL REFERENCES service_areas(id),
  category_id               text NOT NULL REFERENCES ride_categories(id),
  currency                  char(3) NOT NULL DEFAULT 'EUR',
  base_minor                bigint NOT NULL CHECK (base_minor >= 0),
  per_km_minor              bigint NOT NULL CHECK (per_km_minor >= 0),
  per_min_minor             bigint NOT NULL CHECK (per_min_minor >= 0),
  minimum_minor             bigint NOT NULL CHECK (minimum_minor >= 0),
  cancellation_fee_minor    bigint NOT NULL DEFAULT 0,
  cancellation_grace_seconds int NOT NULL DEFAULT 120,
  surge_bp                  int NOT NULL DEFAULT 10000 CHECK (surge_bp BETWEEN 10000 AND 30000),
  commission_bp             int NOT NULL DEFAULT 1500 CHECK (commission_bp BETWEEN 0 AND 5000),
  valid_from                timestamptz NOT NULL DEFAULT now(),
  valid_to                  timestamptz,
  created_at                timestamptz NOT NULL DEFAULT now(),
  UNIQUE (service_area_id, category_id, valid_from)
);
-- Tarifat fillestare për Prishtinën (konfigurim, ndryshohen nga Admin → Çmimet & zonat):
INSERT INTO pricing_rules (service_area_id, category_id, base_minor, per_km_minor, per_min_minor, minimum_minor, cancellation_fee_minor) VALUES
  ('prishtina', 'economy', 100, 45,  8, 200, 100),
  ('prishtina', 'comfort', 150, 65, 10, 300, 150),
  ('prishtina', 'xl',      200, 85, 12, 400, 200),
  ('prishtina', 'taxi',    150, 60, 10, 300, 150);

-- ------------------------------ shoferët ---------------------------------------
CREATE TABLE drivers (
  user_id        uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','suspended')),
  vehicle_make   text NOT NULL,
  vehicle_model  text NOT NULL,
  vehicle_plate  text NOT NULL,
  vehicle_color  text NOT NULL,
  categories     text[] NOT NULL,             -- economy/comfort/xl/taxi që i lejohen
  rating_sum     int NOT NULL DEFAULT 0,
  rating_count   int NOT NULL DEFAULT 0,
  approved_at    timestamptz,
  approved_by    uuid REFERENCES users(id),
  suspended_reason text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX drivers_status_idx ON drivers(status, created_at);

-- ------------------------------ ofertat e çmimit -------------------------------
CREATE TABLE ride_quotes (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id      uuid NOT NULL REFERENCES users(id),
  service_area_id  text NOT NULL REFERENCES service_areas(id),
  category_id      text NOT NULL REFERENCES ride_categories(id),
  pricing_rule_id  uuid NOT NULL REFERENCES pricing_rules(id),
  pickup_lat       double precision NOT NULL,
  pickup_lng       double precision NOT NULL,
  pickup_address   text,
  dropoff_lat      double precision NOT NULL,
  dropoff_lng      double precision NOT NULL,
  dropoff_address  text,
  distance_m       int NOT NULL,
  duration_s       int NOT NULL,
  price_minor      bigint NOT NULL,
  currency         char(3) NOT NULL,
  surge_bp         int NOT NULL,
  expires_at       timestamptz NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ride_quotes_customer_idx ON ride_quotes(customer_id, created_at DESC);

-- ------------------------------ udhëtimet --------------------------------------
CREATE TABLE rides (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id            uuid NOT NULL REFERENCES users(id),
  driver_id              uuid REFERENCES drivers(user_id),
  quote_id               uuid NOT NULL REFERENCES ride_quotes(id),
  service_area_id        text NOT NULL REFERENCES service_areas(id),
  category_id            text NOT NULL REFERENCES ride_categories(id),
  state                  text NOT NULL CHECK (state IN (
                           'matching','assigned','arrived','in_progress','completed','cancelled','no_driver')),
  payment_method         text NOT NULL CHECK (payment_method IN ('cash','wallet','card')),
  payment_status         text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending','paid','failed','cash','none')),
  pickup_lat             double precision NOT NULL,
  pickup_lng             double precision NOT NULL,
  pickup_address         text,
  dropoff_lat            double precision NOT NULL,
  dropoff_lng            double precision NOT NULL,
  dropoff_address        text,
  distance_m             int NOT NULL,
  duration_s             int NOT NULL,
  price_quoted_minor     bigint NOT NULL,
  price_final_minor      bigint,
  commission_minor       bigint,
  cancellation_fee_minor bigint NOT NULL DEFAULT 0,
  currency               char(3) NOT NULL,
  note                   text,
  matching_attempts      int NOT NULL DEFAULT 0,
  cancelled_by           text CHECK (cancelled_by IN ('customer','driver','system')),
  cancellation_reason    text,
  idempotency_key        text NOT NULL,
  requested_at           timestamptz NOT NULL DEFAULT now(),
  assigned_at            timestamptz,
  arrived_at             timestamptz,
  started_at             timestamptz,
  completed_at           timestamptz,
  cancelled_at           timestamptz,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (customer_id, idempotency_key)
);
CREATE INDEX rides_customer_idx ON rides(customer_id, created_at DESC);
CREATE INDEX rides_driver_idx   ON rides(driver_id, created_at DESC) WHERE driver_id IS NOT NULL;
-- një udhëtim aktiv për klient dhe një për shofer (mbrojtje në DB, jo vetëm në kod)
CREATE UNIQUE INDEX rides_customer_active_idx ON rides(customer_id)
  WHERE state IN ('matching','assigned','arrived','in_progress');
CREATE UNIQUE INDEX rides_driver_active_idx ON rides(driver_id)
  WHERE state IN ('assigned','arrived','in_progress');
CREATE INDEX rides_matching_idx ON rides(requested_at) WHERE state = 'matching';

CREATE TABLE ride_events (
  id          bigserial PRIMARY KEY,
  ride_id     uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
  from_state  text,
  to_state    text NOT NULL,
  actor_type  text NOT NULL CHECK (actor_type IN ('customer','driver','system')),
  actor_id    uuid,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ride_events_ride_idx ON ride_events(ride_id, created_at);

-- ------------------------------ ofertat e dispatch-it ---------------------------
CREATE TABLE ride_offers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ride_id      uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
  driver_id    uuid NOT NULL REFERENCES drivers(user_id),
  round        int NOT NULL,
  state        text NOT NULL DEFAULT 'offered' CHECK (state IN ('offered','accepted','declined','expired','withdrawn')),
  distance_m   int NOT NULL,                  -- shoferi → pikë marrjeje (vijë ajrore) në momentin e ofertës
  eta_s        int NOT NULL,
  expires_at   timestamptz NOT NULL,
  responded_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (ride_id, driver_id)
);
CREATE INDEX ride_offers_open_driver_idx ON ride_offers(driver_id, expires_at) WHERE state = 'offered';
CREATE INDEX ride_offers_open_expiry_idx ON ride_offers(expires_at) WHERE state = 'offered';

-- ------------------------------ lokacioni (persistencë selektive) ----------------
-- Burimi i gjallë është Redis GEO (§27); këtu ruhen vetëm mostra gjatë udhëtimit (~çdo 30 s) për
-- rrugën e udhëtimit, mosmarrëveshjet dhe sigurinë. Kurrë çdo GPS update.
CREATE TABLE driver_location_samples (
  id          bigserial PRIMARY KEY,
  driver_id   uuid NOT NULL REFERENCES drivers(user_id),
  ride_id     uuid REFERENCES rides(id) ON DELETE SET NULL,
  lat         double precision NOT NULL,
  lng         double precision NOT NULL,
  heading     real,
  speed_mps   real,
  recorded_at timestamptz NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX driver_location_samples_ride_idx ON driver_location_samples(ride_id, recorded_at) WHERE ride_id IS NOT NULL;
