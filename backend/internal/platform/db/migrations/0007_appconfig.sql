-- =============================================================================
-- KREJT — 0007: konfigurimi i platformës — versionet e aplikacioneve (§64), feature flags (§65).
-- =============================================================================

CREATE TABLE app_versions (
  app                  text NOT NULL CHECK (app IN ('customer','driver','partner','pro','business','admin')),
  platform             text NOT NULL CHECK (platform IN ('ios','android','web')),
  min_version          text NOT NULL DEFAULT '0.0.0',   -- nën këtë: update i detyrueshëm
  recommended_version  text NOT NULL DEFAULT '0.0.0',   -- nën këtë: sugjerim update-i
  maintenance          boolean NOT NULL DEFAULT false,
  maintenance_message  text,                            -- çelës përkthimi ose tekst i shkurtër
  updated_by           uuid REFERENCES users(id),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (app, platform)
);
INSERT INTO app_versions (app, platform) VALUES
  ('customer','ios'), ('customer','android'), ('driver','ios'), ('driver','android'),
  ('partner','android'), ('partner','web'), ('pro','ios'), ('pro','android'), ('business','web'), ('admin','web');

CREATE TABLE feature_flags (
  key              text PRIMARY KEY,                    -- p.sh. rides.request, wallet.topup, rides.surge_dynamic
  enabled          boolean NOT NULL DEFAULT false,
  rollout_percent  smallint NOT NULL DEFAULT 100 CHECK (rollout_percent BETWEEN 0 AND 100),
  public           boolean NOT NULL DEFAULT false,      -- i dukshëm te GET /config (klientët)
  description      text,
  updated_by       uuid REFERENCES users(id),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
-- Flag-et e V1 (ndryshohen nga Admin → Feature flags; asnjë flag i shpërndarë rastësisht në kod)
INSERT INTO feature_flags (key, enabled, public, description) VALUES
  ('rides.request',          true,  true,  'Kërkesa për udhëtim (çaktivizim emergjent i shërbimit)'),
  ('rides.surge_dynamic',    false, false, 'Surge dinamik sipas kërkesës (rregullat mbajnë surge statik derisa të aktivizohet)'),
  ('wallet.topup',           true,  true,  'Mbushja e wallet-it me kartë'),
  ('drivers.self_signup',    true,  true,  'Aplikimi i shoferëve nga aplikacioni'),
  ('notifications.push',     true,  false, 'Dërgimi i push-eve (çaktivizim emergjent)');
