-- =============================================================================
-- KREJT — 0006: pagesat me kartë (§24): qëllime pagese (payment intents) përmes ofruesit,
-- webhook-e të nënshkruara me mbrojtje nga dublikatat, rimbursime. V1: mbushje e wallet-it të mbyllur (§5).
-- =============================================================================

CREATE TABLE payment_intents (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             uuid NOT NULL REFERENCES users(id),
  purpose             text NOT NULL CHECK (purpose IN ('wallet_topup')),   -- V1: vetëm mbushje; asnjë P2P
  amount_minor        bigint NOT NULL CHECK (amount_minor > 0),
  currency            char(3) NOT NULL DEFAULT 'EUR',
  provider            text NOT NULL,                                        -- stripe | raiffeisen | devlog (vetëm dev)
  provider_intent_id  text UNIQUE,
  status              text NOT NULL DEFAULT 'created' CHECK (status IN (
                        'created','requires_action','processing','succeeded','failed','canceled')),
  failure_code        text,
  idempotency_key     text NOT NULL,
  metadata            jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  succeeded_at        timestamptz,
  UNIQUE (user_id, idempotency_key)
);
CREATE INDEX payment_intents_user_idx ON payment_intents(user_id, created_at DESC);
CREATE INDEX payment_intents_open_idx ON payment_intents(created_at) WHERE status IN ('created','requires_action','processing');

-- çdo webhook regjistrohet një herë (id-ja e ngjarjes së ofruesit) — ridorëzimet nuk përpunohen dy herë
CREATE TABLE payment_webhook_events (
  provider     text NOT NULL,
  event_id     text NOT NULL,
  event_type   text NOT NULL,
  intent_id    uuid REFERENCES payment_intents(id),
  received_at  timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  error        text,
  PRIMARY KEY (provider, event_id)
);

CREATE TABLE payment_refunds (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  intent_id           uuid NOT NULL REFERENCES payment_intents(id),
  amount_minor        bigint NOT NULL CHECK (amount_minor > 0),
  reason              text,
  provider_refund_id  text UNIQUE,
  status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','succeeded','failed')),
  requested_by        uuid REFERENCES users(id),
  idempotency_key     text NOT NULL UNIQUE,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now()
);
