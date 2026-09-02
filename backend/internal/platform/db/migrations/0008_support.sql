-- =============================================================================
-- KREJT — 0008: mbështetja (§36): tiketa me mesazhe, raportet e sigurisë (SOS) me prioritet urgjent.
-- =============================================================================

CREATE TABLE support_tickets (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category         text NOT NULL CHECK (category IN ('ride','order','payment','refund','account','safety','other')),
  subject          text NOT NULL,
  status           text NOT NULL DEFAULT 'open' CHECK (status IN ('open','pending_user','resolved','closed')),
  priority         text NOT NULL DEFAULT 'normal' CHECK (priority IN ('normal','high','urgent')),
  ride_id          uuid REFERENCES rides(id) ON DELETE SET NULL,
  assigned_to      uuid REFERENCES users(id),
  last_message_at  timestamptz NOT NULL DEFAULT now(),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  resolved_at      timestamptz
);
CREATE INDEX support_tickets_user_idx ON support_tickets(user_id, created_at DESC);
CREATE INDEX support_tickets_queue_idx ON support_tickets(status, priority, last_message_at);
CREATE INDEX support_tickets_assigned_idx ON support_tickets(assigned_to) WHERE status IN ('open','pending_user');

CREATE TABLE support_messages (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id    uuid NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
  author_id    uuid REFERENCES users(id),
  author_role  text NOT NULL CHECK (author_role IN ('user','support','system')),
  body         text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX support_messages_ticket_idx ON support_messages(ticket_id, created_at);

CREATE TABLE safety_reports (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_id  uuid NOT NULL REFERENCES users(id),
  ride_id      uuid REFERENCES rides(id) ON DELETE SET NULL,
  ticket_id    uuid REFERENCES support_tickets(id) ON DELETE SET NULL,
  kind         text NOT NULL CHECK (kind IN ('sos','unsafe_driving','harassment','accident','vehicle_issue','other')),
  lat          double precision,
  lng          double precision,
  description  text,
  status       text NOT NULL DEFAULT 'open' CHECK (status IN ('open','reviewing','closed')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX safety_reports_open_idx ON safety_reports(created_at) WHERE status <> 'closed';
