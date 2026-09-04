-- Kupona zbritjeje (§35): kodi shkruhet i normalizuar (shkronja të mëdha, vetëm shkronja/shifra).
-- Zbritja llogaritet vetëm në server; kostoja i bie platformës (llogaria krejt:marketing).

CREATE TABLE coupons (
  code                text PRIMARY KEY,
  kind                text NOT NULL CHECK (kind IN ('percent','fixed')),
  percent_bp          int NOT NULL DEFAULT 0 CHECK (percent_bp BETWEEN 0 AND 10000),
  amount_minor        bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
  min_order_minor     bigint NOT NULL DEFAULT 0 CHECK (min_order_minor >= 0),
  scope               text NOT NULL DEFAULT 'all' CHECK (scope IN ('all','food','parcels')),
  starts_at           timestamptz,
  ends_at             timestamptz,
  max_uses            int,
  max_uses_per_user   int,
  uses_count          int NOT NULL DEFAULT 0,
  active              boolean NOT NULL DEFAULT true,
  note                text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE coupon_redemptions (
  id              bigserial PRIMARY KEY,
  coupon_code     text NOT NULL REFERENCES coupons(code),
  user_id         uuid NOT NULL REFERENCES users(id),
  reference       text NOT NULL,        -- order:{id} | parcel:{id}
  discount_minor  bigint NOT NULL CHECK (discount_minor >= 0),
  created_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (reference)
);
CREATE INDEX coupon_redemptions_user_idx ON coupon_redemptions(coupon_code, user_id);

ALTER TABLE orders ADD COLUMN coupon_code text REFERENCES coupons(code);
ALTER TABLE parcels ADD COLUMN discount_minor bigint NOT NULL DEFAULT 0 CHECK (discount_minor >= 0);
ALTER TABLE parcels ADD COLUMN coupon_code text REFERENCES coupons(code);

INSERT INTO ledger_accounts (code, owner_type, kind) VALUES ('krejt:marketing', 'platform', 'expense');
