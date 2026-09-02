-- =============================================================================
-- KREJT — 0002: profili & adresat e përdoruesit (§16 Profile, §46 harta, §57 forma),
-- preferencat e njoftimeve (§29), riprovimi i outbox-it (§41).
-- Vetëm Kosovë (§1): adresat kontrollohen server-side që të jenë brenda kufijve.
-- =============================================================================

-- ------------------------------ outbox: riprovim ------------------------------
ALTER TABLE outbox_events
  ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN last_error      text;
DROP INDEX IF EXISTS outbox_unpublished_idx;
CREATE INDEX outbox_unpublished_idx ON outbox_events(next_attempt_at, created_at) WHERE published_at IS NULL;

-- ------------------------------ adresat e ruajtura ----------------------------
CREATE TABLE user_addresses (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label         text NOT NULL CHECK (label IN ('home','work','other')),
  name          text,                                     -- emërtim i lirë ("Te gjyshja")
  line1         text NOT NULL,
  line2         text,
  city          text NOT NULL,
  postal_code   text,
  lat           double precision NOT NULL CHECK (lat BETWEEN 41.85 AND 43.30),  -- Kosovë (§1)
  lng           double precision NOT NULL CHECK (lng BETWEEN 19.95 AND 21.80),
  place_id      text,                                     -- Google Places, pas MapProvider (§46)
  instructions  text,                                     -- udhëzime për shoferin/korrierin
  is_default    boolean NOT NULL DEFAULT false,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE INDEX user_addresses_user_idx ON user_addresses(user_id, created_at) WHERE deleted_at IS NULL;
-- vetëm një adresë parazgjedhje aktive për përdorues
CREATE UNIQUE INDEX user_addresses_default_idx ON user_addresses(user_id) WHERE is_default AND deleted_at IS NULL;

-- ------------------------------ preferencat e njoftimeve ----------------------
CREATE TABLE notification_preferences (
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category    text NOT NULL CHECK (category IN (
                'security','rides','orders','payments','wallet','loyalty','promotions','support')),
  push        boolean NOT NULL DEFAULT true,
  email       boolean NOT NULL DEFAULT true,
  sms         boolean NOT NULL DEFAULT false,
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, category),
  -- njoftimet e sigurisë (kyçje e re, pajisje e re, ndryshim profili) nuk çaktivizohen dot (§51)
  CONSTRAINT security_push_required CHECK (category <> 'security' OR push)
);
