-- =============================================================================
-- KREJT — 0009: chat klient ↔ shofer brenda udhëtimit (§28): tekst, kohë, gjendje leximi,
-- ruajtje e kufizuar (pastrohet pas 90 ditësh — §28 retention).
-- =============================================================================

CREATE TABLE ride_chat_messages (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ride_id      uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
  sender_id    uuid NOT NULL REFERENCES users(id),
  sender_role  text NOT NULL CHECK (sender_role IN ('customer','driver')),
  body         text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  read_at      timestamptz
);
CREATE INDEX ride_chat_ride_idx ON ride_chat_messages(ride_id, created_at);
CREATE INDEX ride_chat_retention_idx ON ride_chat_messages(created_at);
