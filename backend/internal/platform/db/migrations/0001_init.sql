-- =============================================================================
-- KREJT — 0001: themeli. Identiteti (§37, §53), sesionet, ledger-i (§23 — para si numra
-- të plotë në cent, hyrje të pandryshueshme, debi = kredi), outbox (§41), audit (§35).
-- Vetëm Kosovë, vetëm EUR në V1 — por currency është kolonë që dita e parë.
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ----------------------------- identiteti ------------------------------------
CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone_e164    text UNIQUE,                       -- +383…, +49… (diaspora)
  email         citext UNIQUE,
  full_name     text,
  locale        text NOT NULL DEFAULT 'sq' CHECK (locale IN ('sq','en','de')),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked','deleted')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);

-- kapacitetet: një llogari, shumë role (§37) — autorizimi kontrollohet gjithmonë në server
CREATE TABLE user_capabilities (
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  capability  text NOT NULL CHECK (capability IN (
                'CUSTOMER','RIDE_DRIVER','TAXI_DRIVER','FOOD_COURIER','PACKAGE_COURIER',
                'MERCHANT','RESTAURANT','STORE','PHARMACY','PROFESSIONAL','BUSINESS',
                'ADMIN','SUPPORT','FINANCE','OPERATIONS','SUPER_ADMIN')),
  granted_at  timestamptz NOT NULL DEFAULT now(),
  granted_by  uuid REFERENCES users(id),
  revoked_at  timestamptz,
  PRIMARY KEY (user_id, capability)
);

-- sesionet & pajisjet (§53): refresh token me rotacion, i ruajtur si hash
CREATE TABLE sessions (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id          text NOT NULL,
  device_name        text,
  platform           text CHECK (platform IN ('ios','android','web')),
  refresh_token_hash bytea NOT NULL,
  refresh_expires_at timestamptz NOT NULL,
  last_seen_at       timestamptz NOT NULL DEFAULT now(),
  ip                 inet,
  revoked_at         timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions(user_id) WHERE revoked_at IS NULL;

-- kodet OTP: vetëm hash, TTL, kufi përpjekjesh (mbrojtje brute-force §51)
CREATE TABLE otp_challenges (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phone_e164   text NOT NULL,
  code_hash    bytea NOT NULL,
  channel      text NOT NULL DEFAULT 'sms' CHECK (channel IN ('sms','whatsapp')),
  attempts     smallint NOT NULL DEFAULT 0,
  expires_at   timestamptz NOT NULL,
  consumed_at  timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX otp_phone_idx ON otp_challenges(phone_e164, created_at DESC);

-- ----------------------------- ledger (§23) -----------------------------------
-- Llogaritë: user:{id}:wallet, driver:{id}:earnings, merchant:{id}:payable, krejt:commission, krejt:cash_clearing …
CREATE TABLE ledger_accounts (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code        text NOT NULL UNIQUE,
  owner_type  text NOT NULL CHECK (owner_type IN ('user','driver','courier','merchant','professional','business','platform')),
  owner_id    uuid,
  kind        text NOT NULL CHECK (kind IN ('asset','liability','revenue','expense','clearing')),
  currency    char(3) NOT NULL DEFAULT 'EUR',
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ledger_transactions (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind             text NOT NULL,                 -- ride_payment, order_payment, refund, cashback, payout, commission …
  reference        text NOT NULL,                 -- p.sh. ride:R-77412, order:F-10482
  idempotency_key  text NOT NULL UNIQUE,          -- riprovimi nuk krijon kurrë transaksion të dyfishtë (§24)
  currency         char(3) NOT NULL DEFAULT 'EUR',
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_tx_reference_idx ON ledger_transactions(reference);

CREATE TABLE ledger_entries (
  id            bigserial PRIMARY KEY,
  tx_id         uuid NOT NULL REFERENCES ledger_transactions(id),
  account_id    uuid NOT NULL REFERENCES ledger_accounts(id),
  debit_minor   bigint NOT NULL DEFAULT 0 CHECK (debit_minor >= 0),
  credit_minor  bigint NOT NULL DEFAULT 0 CHECK (credit_minor >= 0),
  currency      char(3) NOT NULL DEFAULT 'EUR',
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK ((debit_minor > 0 AND credit_minor = 0) OR (credit_minor > 0 AND debit_minor = 0))
);
CREATE INDEX ledger_entries_account_idx ON ledger_entries(account_id, created_at);
CREATE INDEX ledger_entries_tx_idx ON ledger_entries(tx_id);

-- Rregulli i artë: për çdo transaksion SUM(debit) = SUM(credit). Kontrollohet në COMMIT.
CREATE OR REPLACE FUNCTION ledger_assert_balanced() RETURNS trigger AS $$
DECLARE d bigint; c bigint;
BEGIN
  SELECT COALESCE(SUM(debit_minor),0), COALESCE(SUM(credit_minor),0) INTO d, c
    FROM ledger_entries WHERE tx_id = NEW.tx_id;
  IF d <> c THEN
    RAISE EXCEPTION 'ledger transaction % is not balanced: debit=% credit=%', NEW.tx_id, d, c
      USING ERRCODE = 'integrity_constraint_violation';
  END IF;
  RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER ledger_balanced
  AFTER INSERT ON ledger_entries
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION ledger_assert_balanced();

-- Append-only: hyrjet dhe transaksionet nuk ndryshohen dhe nuk fshihen kurrë (§23, §40).
CREATE OR REPLACE FUNCTION ledger_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'ledger rows are immutable (%.%)', TG_TABLE_NAME, TG_OP
    USING ERRCODE = 'insufficient_privilege';
END $$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_entries_immutable BEFORE UPDATE OR DELETE ON ledger_entries
  FOR EACH ROW EXECUTE FUNCTION ledger_immutable();
CREATE TRIGGER ledger_transactions_immutable BEFORE UPDATE OR DELETE ON ledger_transactions
  FOR EACH ROW EXECUTE FUNCTION ledger_immutable();

-- Llogaritë e platformës
INSERT INTO ledger_accounts (code, owner_type, kind) VALUES
  ('krejt:commission',    'platform', 'revenue'),
  ('krejt:cash_clearing', 'platform', 'clearing'),
  ('krejt:card_clearing', 'platform', 'clearing'),
  ('krejt:cashback',      'platform', 'expense'),
  ('krejt:refunds',       'platform', 'expense');

-- ----------------------------- outbox (§41) -----------------------------------
CREATE TABLE outbox_events (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  aggregate_type text NOT NULL,
  aggregate_id   text NOT NULL,
  event_type     text NOT NULL,                   -- UserCreated, RideRequested, PaymentCaptured …
  payload        jsonb NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  published_at   timestamptz,
  attempts       smallint NOT NULL DEFAULT 0
);
CREATE INDEX outbox_unpublished_idx ON outbox_events(created_at) WHERE published_at IS NULL;

-- ----------------------------- audit (§35, §50) --------------------------------
CREATE TABLE audit_log (
  id          bigserial PRIMARY KEY,
  actor_id    uuid,
  actor_type  text NOT NULL DEFAULT 'user',
  action      text NOT NULL,
  target_type text,
  target_id   text,
  ip          inet,
  request_id  text,
  metadata    jsonb,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_target_idx ON audit_log(target_type, target_id, created_at DESC);
CREATE TRIGGER audit_log_immutable BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION ledger_immutable();

-- ----------------------------- idempotency (§24) -------------------------------
CREATE TABLE idempotency_keys (
  key            text NOT NULL,
  user_id        uuid NOT NULL,
  request_hash   bytea NOT NULL,
  response_code  int,
  response_body  jsonb,
  created_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, key)
);
CREATE INDEX idempotency_created_idx ON idempotency_keys(created_at); -- pastrohen pas 24 h
