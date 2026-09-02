-- =============================================================================
-- KREJT — 0014 (Faza 2): porositë e ushqimit/marketit (§19, §21): shporta → checkout → përgatitje →
-- korrier → dorëzim, me çmim server-side, makinë gjendjesh dhe shlyerje në ledger (cash / wallet).
-- =============================================================================

INSERT INTO ledger_accounts (code, owner_type, kind) VALUES ('krejt:delivery_fees', 'platform', 'revenue')
  ON CONFLICT (code) DO NOTHING;

CREATE TABLE orders (
  id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                   text NOT NULL UNIQUE,                -- kod i shkurtër për klientin/merchant-in: "K7F3QA"
  customer_id            uuid NOT NULL REFERENCES users(id),
  merchant_id            uuid NOT NULL REFERENCES merchants(id),
  courier_id             uuid REFERENCES drivers(user_id),
  state                  text NOT NULL CHECK (state IN (
                           'pending_merchant','accepted','preparing','ready','courier_assigned','picked_up','delivered','cancelled','rejected')),
  fulfillment            text NOT NULL CHECK (fulfillment IN ('courier','merchant_delivers','pickup')),
  payment_method         text NOT NULL CHECK (payment_method IN ('cash','wallet','card')),
  payment_status         text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending','paid','failed','cash','refunded','none')),
  items_total_minor      bigint NOT NULL CHECK (items_total_minor >= 0),
  delivery_fee_minor     bigint NOT NULL DEFAULT 0,
  discount_minor         bigint NOT NULL DEFAULT 0,
  total_minor            bigint NOT NULL CHECK (total_minor >= 0),
  commission_minor       bigint,                              -- pjesa e KREJT nga items_total (vendoset në dorëzim)
  currency               char(3) NOT NULL DEFAULT 'EUR',
  address_line1          text,
  address_lat            double precision,
  address_lng            double precision,
  address_instructions   text,
  note                   text,
  prep_time_min          int NOT NULL DEFAULT 20,
  ready_at_estimate      timestamptz,
  cancelled_by           text CHECK (cancelled_by IN ('customer','merchant','system')),
  cancellation_reason    text,
  idempotency_key        text NOT NULL,
  created_at             timestamptz NOT NULL DEFAULT now(),
  accepted_at            timestamptz,
  ready_at               timestamptz,
  picked_up_at           timestamptz,
  delivered_at           timestamptz,
  cancelled_at           timestamptz,
  updated_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (customer_id, idempotency_key)
);
CREATE INDEX orders_customer_idx ON orders(customer_id, created_at DESC);
CREATE INDEX orders_merchant_idx ON orders(merchant_id, created_at DESC);
CREATE INDEX orders_courier_idx ON orders(courier_id, created_at DESC) WHERE courier_id IS NOT NULL;
CREATE INDEX orders_open_merchant_idx ON orders(merchant_id, created_at) WHERE state IN ('pending_merchant','accepted','preparing','ready');
CREATE INDEX orders_dispatch_idx ON orders(ready_at) WHERE state = 'ready' AND fulfillment = 'courier';
-- një porosi aktive për korrier (si te udhëtimet)
CREATE UNIQUE INDEX orders_courier_active_idx ON orders(courier_id)
  WHERE courier_id IS NOT NULL AND state IN ('courier_assigned','picked_up');

CREATE TABLE order_items (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id         uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id       uuid NOT NULL REFERENCES products(id),
  name             text NOT NULL,                             -- fotografia e emrit/çmimit në momentin e porosisë
  options          text[] NOT NULL DEFAULT '{}',
  option_ids       uuid[] NOT NULL DEFAULT '{}',
  unit_minor       bigint NOT NULL,
  quantity         int NOT NULL CHECK (quantity > 0),
  total_minor      bigint NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_items_order_idx ON order_items(order_id);

CREATE TABLE order_events (
  id          bigserial PRIMARY KEY,
  order_id    uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  from_state  text,
  to_state    text NOT NULL,
  actor_type  text NOT NULL CHECK (actor_type IN ('customer','merchant','courier','system')),
  actor_id    uuid,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_events_order_idx ON order_events(order_id, created_at);

CREATE TABLE order_offers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id     uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  courier_id   uuid NOT NULL REFERENCES drivers(user_id),
  round        int NOT NULL,
  state        text NOT NULL DEFAULT 'offered' CHECK (state IN ('offered','accepted','declined','expired','withdrawn')),
  distance_m   int NOT NULL,
  eta_s        int NOT NULL,
  expires_at   timestamptz NOT NULL,
  responded_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (order_id, courier_id)
);
CREATE INDEX order_offers_open_courier_idx ON order_offers(courier_id, expires_at) WHERE state = 'offered';
CREATE INDEX order_offers_open_expiry_idx ON order_offers(expires_at) WHERE state = 'offered';
