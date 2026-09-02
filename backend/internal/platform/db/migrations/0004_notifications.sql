-- =============================================================================
-- KREJT — 0004: njoftimet (§29, §47): token-at e push-it (regjistrim, rifreskim, të pavlefshëm),
-- kutia e njoftimeve në aplikacion, gjurma e dorëzimit për kanal.
-- =============================================================================

CREATE TABLE push_tokens (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_id   uuid REFERENCES sessions(id) ON DELETE SET NULL,
  platform     text NOT NULL CHECK (platform IN ('ios','android','web')),
  provider     text NOT NULL DEFAULT 'fcm',
  token        text NOT NULL UNIQUE,
  locale       text NOT NULL DEFAULT 'sq' CHECK (locale IN ('sq','en','de')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  invalid_at   timestamptz                             -- FCM: UNREGISTERED / INVALID_ARGUMENT
);
CREATE INDEX push_tokens_user_idx ON push_tokens(user_id) WHERE invalid_at IS NULL;

CREATE TABLE notifications (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_id    uuid NOT NULL,                             -- id-ja e ngjarjes së outbox-it (idempotencë)
  event_type  text NOT NULL,
  category    text NOT NULL CHECK (category IN (
                'security','rides','orders','payments','wallet','loyalty','promotions','support')),
  title_key   text NOT NULL,                             -- çelësa përkthimi + parametra (klienti i rendit vetë)
  body_key    text NOT NULL,
  params      jsonb NOT NULL DEFAULT '{}'::jsonb,
  deep_link   text,                                      -- krejt://rides/{id} …
  read_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (event_id, user_id)
);
CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read_at IS NULL;

CREATE TABLE notification_deliveries (
  id                  bigserial PRIMARY KEY,
  notification_id     uuid NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
  channel             text NOT NULL CHECK (channel IN ('push','sms','email')),
  status              text NOT NULL CHECK (status IN ('sent','failed','skipped')),
  target              text,                              -- token i shkurtuar / numri i maskuar (kurrë i plotë)
  provider_message_id text,
  error               text,
  created_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notification_deliveries_notif_idx ON notification_deliveries(notification_id);
