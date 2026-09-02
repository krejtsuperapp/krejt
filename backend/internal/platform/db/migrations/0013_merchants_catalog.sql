-- =============================================================================
-- KREJT — 0013 (Faza 2): merchant-ët (restorante, dyqane, barnatore, ushqimore §19, §21), oraret, stafi,
-- katalogu (kategori, produkte, modifikues me rregulla min/max, disponueshmëri). Vetëm Kosovë.
-- =============================================================================

CREATE TABLE merchants (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id       uuid NOT NULL REFERENCES users(id),
  type                text NOT NULL CHECK (type IN ('restaurant','store','grocery','pharmacy')),
  name                text NOT NULL,
  slug                text NOT NULL UNIQUE,
  description         text,
  phone               text,
  address_line1       text NOT NULL,
  city                text NOT NULL,
  lat                 double precision NOT NULL CHECK (lat BETWEEN 41.85 AND 43.30),
  lng                 double precision NOT NULL CHECK (lng BETWEEN 19.95 AND 21.80),
  service_area_id     text REFERENCES service_areas(id),
  status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','paused','suspended')),
  cuisines            text[] NOT NULL DEFAULT '{}',        -- restorante: 'burger','pizza','traditional' …
  tags                text[] NOT NULL DEFAULT '{}',
  fulfillment_mode    text NOT NULL DEFAULT 'courier' CHECK (fulfillment_mode IN ('courier','merchant_delivers','pickup')),
  min_order_minor     bigint NOT NULL DEFAULT 0,
  delivery_fee_minor  bigint NOT NULL DEFAULT 150,         -- tarifë bazë; llogaritja me shkallë vjen me orders
  prep_time_min       int NOT NULL DEFAULT 20,
  commission_bp       int NOT NULL DEFAULT 1500 CHECK (commission_bp BETWEEN 0 AND 5000),
  rating_sum          int NOT NULL DEFAULT 0,
  rating_count        int NOT NULL DEFAULT 0,
  logo_key            text,
  cover_key           text,
  accepting_orders    boolean NOT NULL DEFAULT true,        -- "pauzë" e shpejtë nga merchant-i (brenda orarit)
  approved_at         timestamptz,
  approved_by         uuid REFERENCES users(id),
  suspended_reason    text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX merchants_status_idx ON merchants(status, type);
CREATE INDEX merchants_area_idx ON merchants(service_area_id) WHERE status = 'active';
CREATE INDEX merchants_name_trgm_idx ON merchants USING gin (unaccent(name) gin_trgm_ops);

CREATE TABLE merchant_hours (
  merchant_id  uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  weekday      smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),   -- 0 = e diel (ISO: 1 = e hënë … 7) — këtu 0..6 si Go time.Weekday
  opens        time NOT NULL,
  closes       time NOT NULL,                                       -- closes <= opens → kalon mesnatën
  PRIMARY KEY (merchant_id, weekday, opens)
);

CREATE TABLE merchant_staff (
  merchant_id  uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role         text NOT NULL CHECK (role IN ('owner','manager','staff')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (merchant_id, user_id)
);
CREATE INDEX merchant_staff_user_idx ON merchant_staff(user_id);

CREATE TABLE catalog_categories (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id  uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  name         text NOT NULL,
  sort         int NOT NULL DEFAULT 0,
  active       boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX catalog_categories_merchant_idx ON catalog_categories(merchant_id, sort);

CREATE TABLE products (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  merchant_id   uuid NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
  category_id   uuid REFERENCES catalog_categories(id) ON DELETE SET NULL,
  name          text NOT NULL,
  description   text,
  price_minor   bigint NOT NULL CHECK (price_minor >= 0),
  currency      char(3) NOT NULL DEFAULT 'EUR',
  image_key     text,
  available     boolean NOT NULL DEFAULT true,
  unit          text NOT NULL DEFAULT 'piece' CHECK (unit IN ('piece','kg','g','l','ml','portion')),
  tags          text[] NOT NULL DEFAULT '{}',              -- 'vegan','spicy','otc' …
  sort          int NOT NULL DEFAULT 0,
  deleted_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_merchant_idx ON products(merchant_id, category_id, sort) WHERE deleted_at IS NULL;
CREATE INDEX products_name_trgm_idx ON products USING gin (unaccent(name) gin_trgm_ops);

CREATE TABLE modifier_groups (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id  uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
  name        text NOT NULL,                                -- "Madhësia", "Shtesa"
  min_select  smallint NOT NULL DEFAULT 0 CHECK (min_select >= 0),
  max_select  smallint NOT NULL DEFAULT 1 CHECK (max_select >= 1),
  sort        int NOT NULL DEFAULT 0,
  CHECK (min_select <= max_select)
);
CREATE INDEX modifier_groups_product_idx ON modifier_groups(product_id, sort);

CREATE TABLE modifier_options (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  group_id           uuid NOT NULL REFERENCES modifier_groups(id) ON DELETE CASCADE,
  name               text NOT NULL,
  price_delta_minor  bigint NOT NULL DEFAULT 0,
  available          boolean NOT NULL DEFAULT true,
  sort               int NOT NULL DEFAULT 0
);
CREATE INDEX modifier_options_group_idx ON modifier_options(group_id, sort);
