-- =============================================================================
-- KREJT — 0010: fraud / risk v1 (§67): flamuj risku nga rregulla mbi ngjarjet, bllokim përdoruesish nga
-- Operacionet me arsye dhe audit. Rregullat vetëm sinjalizojnë (njerëzit vendosin), përveç kufijve të shpejtësisë.
-- =============================================================================

CREATE TABLE risk_flags (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         text NOT NULL,                        -- driver_cancel_rate, customer_cancel_burst, review_burst, topup_velocity, refund_pattern, …
  severity     text NOT NULL CHECK (severity IN ('low','medium','high')),
  score        int NOT NULL DEFAULT 0,
  details      jsonb NOT NULL DEFAULT '{}'::jsonb,
  source_event uuid,                                 -- ngjarja e outbox-it që e shkaktoi (gjurmueshmëri)
  status       text NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','dismissed','confirmed')),
  resolved_by  uuid REFERENCES users(id),
  resolved_at  timestamptz,
  note         text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, kind, source_event)
);
CREATE INDEX risk_flags_open_idx ON risk_flags(severity, created_at DESC) WHERE status IN ('open','reviewing');
CREATE INDEX risk_flags_user_idx ON risk_flags(user_id, created_at DESC);

-- bllokimi i përdoruesit: arsyeja dhe kush e bëri (users.status = 'blocked' mbetet burimi i së vërtetës)
ALTER TABLE users
  ADD COLUMN blocked_reason text,
  ADD COLUMN blocked_at     timestamptz,
  ADD COLUMN blocked_by     uuid REFERENCES users(id);
