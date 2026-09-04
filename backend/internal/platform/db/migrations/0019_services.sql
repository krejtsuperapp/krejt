-- Shërbimet (§22): klienti përshkruan punën, mjeshtrit e miratuar dërgojnë oferta, klienti zgjedh një
-- prej tyre. Çmimin e vendos mjeshtri te oferta — platforma nuk shpik tarifa; komisioni mbahet nga
-- oferta e pranuar, si te udhëtimet.
--
--   open ─► booked ─► in_progress ─► completed
--     │        │            │
--     └────────┴────────────┴──► cancelled        open ─► no_offers (pa ofertë brenda afatit)

CREATE TABLE service_categories (
  id            text PRIMARY KEY,
  name_key      text NOT NULL,
  commission_bp int NOT NULL DEFAULT 1500 CHECK (commission_bp BETWEEN 0 AND 10000),
  sort          int NOT NULL DEFAULT 0,
  active        boolean NOT NULL DEFAULT true
);
INSERT INTO service_categories (id, name_key, sort) VALUES
  ('electrician', 'service.category.electrician', 1),
  ('plumber',     'service.category.plumber',     2),
  ('cleaning',    'service.category.cleaning',    3),
  ('ac',          'service.category.ac',          4),
  ('appliance',   'service.category.appliance',   5),
  ('moving',      'service.category.moving',      6),
  ('handyman',    'service.category.handyman',    7);

-- Mjeshtri: një përdorues i miratuar nga Operacionet, me kategoritë që i lejohen.
CREATE TABLE service_providers (
  user_id        uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','suspended')),
  categories     text[] NOT NULL,
  business_name  text,
  bio            text,
  city           text NOT NULL,
  phone_public   text,
  rating         numeric(2,1),
  rating_count   int NOT NULL DEFAULT 0,
  jobs_done      int NOT NULL DEFAULT 0,
  suspended_reason text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX service_providers_status_idx ON service_providers(status, created_at);

CREATE TABLE service_requests (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                text NOT NULL UNIQUE,
  customer_id         uuid NOT NULL REFERENCES users(id),
  category_id         text NOT NULL REFERENCES service_categories(id),
  provider_id         uuid REFERENCES service_providers(user_id),
  accepted_offer_id   uuid,
  state               text NOT NULL CHECK (state IN ('open','booked','in_progress','completed','cancelled','no_offers')),
  title               text NOT NULL,
  description         text NOT NULL,
  address_line1       text NOT NULL,
  address_lat         double precision NOT NULL,
  address_lng         double precision NOT NULL,
  address_instructions text,
  preferred_at        timestamptz,
  photo_keys          text[] NOT NULL DEFAULT '{}',
  payment_method      text NOT NULL CHECK (payment_method IN ('cash','wallet')),
  payment_status      text NOT NULL DEFAULT 'none' CHECK (payment_status IN ('none','pending','paid','failed','cash')),
  price_minor         bigint,                      -- vjen nga oferta e pranuar
  commission_minor    bigint NOT NULL DEFAULT 0,
  currency            text NOT NULL DEFAULT 'EUR',
  cancelled_by        text CHECK (cancelled_by IN ('customer','provider','ops','system')),
  cancellation_reason text,
  idempotency_key     text NOT NULL,
  created_at          timestamptz NOT NULL DEFAULT now(),
  booked_at           timestamptz,
  started_at          timestamptz,
  completed_at        timestamptz,
  cancelled_at        timestamptz,
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (customer_id, idempotency_key)
);
CREATE INDEX service_requests_customer_idx ON service_requests(customer_id, created_at DESC);
CREATE INDEX service_requests_open_idx ON service_requests(category_id, created_at DESC) WHERE state = 'open';
CREATE INDEX service_requests_provider_idx ON service_requests(provider_id, created_at DESC);

-- Oferta e mjeshtrit: çmimi dhe kur mund të vijë. Një ofertë për mjeshtër për kërkesë.
CREATE TABLE service_offers (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id    uuid NOT NULL REFERENCES service_requests(id) ON DELETE CASCADE,
  provider_id   uuid NOT NULL REFERENCES service_providers(user_id),
  price_minor   bigint NOT NULL CHECK (price_minor > 0),
  currency      text NOT NULL DEFAULT 'EUR',
  note          text,
  can_start_at  timestamptz,
  state         text NOT NULL DEFAULT 'offered' CHECK (state IN ('offered','accepted','declined','withdrawn')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  responded_at  timestamptz,
  UNIQUE (request_id, provider_id)
);
CREATE INDEX service_offers_request_idx ON service_offers(request_id, created_at);
CREATE INDEX service_offers_provider_idx ON service_offers(provider_id, created_at DESC);

ALTER TABLE service_requests
  ADD CONSTRAINT service_requests_accepted_offer_fk FOREIGN KEY (accepted_offer_id) REFERENCES service_offers(id);

CREATE TABLE service_events (
  id          bigserial PRIMARY KEY,
  request_id  uuid NOT NULL REFERENCES service_requests(id) ON DELETE CASCADE,
  from_state  text,
  to_state    text NOT NULL,
  actor_type  text NOT NULL CHECK (actor_type IN ('customer','provider','ops','system')),
  actor_id    uuid,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX service_events_request_idx ON service_events(request_id, created_at);

-- Një mjeshtër nuk mund të ketë dy punë në ecje njëkohësisht.
CREATE UNIQUE INDEX service_requests_one_active_per_provider
  ON service_requests(provider_id) WHERE state IN ('booked','in_progress');
