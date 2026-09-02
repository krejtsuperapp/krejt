-- =============================================================================
-- KREJT — 0011: fitimet dhe payout-et e shoferëve (Faza 1: payout javor në bankë), llogaria bankare
-- e shoferit, grupet e payout-eve të Finance (ledger: wallet-i i shoferit → clearing i payout-eve).
-- =============================================================================

INSERT INTO ledger_accounts (code, owner_type, kind) VALUES ('krejt:payout_clearing', 'platform', 'clearing')
  ON CONFLICT (code) DO NOTHING;

CREATE TABLE driver_bank_accounts (
  driver_id     uuid PRIMARY KEY REFERENCES drivers(user_id) ON DELETE CASCADE,
  holder_name   text NOT NULL,
  iban          text NOT NULL,                       -- XK…(20) — Kosovë; validohet server-side (mod 97)
  bank_name     text,
  verified_at   timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE payout_batches (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  period_start  date NOT NULL,
  period_end    date NOT NULL,
  status        text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','exported','completed')),
  total_minor   bigint NOT NULL DEFAULT 0,
  item_count    int NOT NULL DEFAULT 0,
  created_by    uuid REFERENCES users(id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  exported_at   timestamptz,
  completed_at  timestamptz
);

CREATE TABLE payout_items (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  batch_id       uuid NOT NULL REFERENCES payout_batches(id) ON DELETE CASCADE,
  driver_id      uuid NOT NULL REFERENCES drivers(user_id),
  amount_minor   bigint NOT NULL CHECK (amount_minor > 0),
  currency       char(3) NOT NULL DEFAULT 'EUR',
  iban           text NOT NULL,
  holder_name    text NOT NULL,
  status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed')),
  failure_reason text,
  ledger_tx_id   uuid,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  UNIQUE (batch_id, driver_id)
);
CREATE INDEX payout_items_driver_idx ON payout_items(driver_id, created_at DESC);
